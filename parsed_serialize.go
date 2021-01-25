/*
 * MinIO Cloud Storage, (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package simdjson

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
)

const (
	stringBits        = 14
	stringSize        = 1 << stringBits
	stringmask        = stringSize - 1
	serializedVersion = 2
)

// Serializer allows to serialize parsed json and read it back.
// A Serializer can be reused, but not used concurrently.
type Serializer struct {
	// Compressed strings
	sMsg []byte

	// Uncompressed tags
	tagsBuf []byte
	// Values
	valuesBuf     []byte
	valuesCompBuf []byte
	tagsCompBuf   []byte

	compValues, compTags uint8
	compStrings          uint8
	fasterComp           bool

	// Deduplicated strings
	stringWr     io.Writer
	stringsTable [stringSize]uint32
	stringBuf    []byte

	maxBlockSize uint64
}

// NewSerializer will create and initialize a Serializer.
func NewSerializer() *Serializer {
	initSerializerOnce.Do(initSerializer)
	var s Serializer
	s.CompressMode(CompressDefault)
	s.maxBlockSize = 1 << 31
	return &s
}

type CompressMode uint8

const (
	// CompressNone no compression whatsoever.
	CompressNone CompressMode = iota

	// CompressFast will apply light compression,
	// but will not deduplicate strings which may affect deserialization speed.
	CompressFast

	// CompressDefault applies light compression and deduplicates strings.
	CompressDefault

	// CompressBest
	CompressBest
)

func (s *Serializer) CompressMode(c CompressMode) {
	switch c {
	case CompressNone:
		s.compValues = blockTypeUncompressed
		s.compTags = blockTypeUncompressed
		s.compStrings = blockTypeUncompressed
	case CompressFast:
		s.compValues = blockTypeS2
		s.compTags = blockTypeS2
		s.compStrings = blockTypeS2
		s.fasterComp = true
	case CompressDefault:
		s.compValues = blockTypeS2
		s.compTags = blockTypeS2
		s.compStrings = blockTypeS2
	case CompressBest:
		s.compValues = blockTypeZstd
		s.compTags = blockTypeZstd
		s.compStrings = blockTypeZstd
	default:
		panic("unknown compression mode")
	}
}

func serializeNDStream(dst io.Writer, in <-chan Stream, reuse chan<- *ParsedJson, concurrency int, comp CompressMode) error {
	if concurrency <= 0 {
		concurrency = (runtime.GOMAXPROCS(0) + 1) / 2
	}
	var wg sync.WaitGroup
	wg.Add(concurrency)
	type workload struct {
		pj  *ParsedJson
		dst chan []byte
	}
	var readCh = make(chan workload, concurrency)
	var writeCh = make(chan chan []byte, concurrency)
	dstPool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 64<<10)
		},
	}
	for i := 0; i < concurrency; i++ {
		go func() {
			s := NewSerializer()
			s.CompressMode(comp)
			defer wg.Done()
			for input := range readCh {
				res := s.Serialize(dstPool.Get().([]byte)[:0], *input.pj)
				input.dst <- res
				select {
				case reuse <- input.pj:
				default:
				}
			}
		}()
	}
	var writeErr error
	var wwg sync.WaitGroup
	wwg.Add(1)
	go func() {
		defer wwg.Done()
		for block := range writeCh {
			b := <-block
			var n int
			n, writeErr = dst.Write(b)
			if n != len(b) {
				writeErr = io.ErrShortWrite
			}
		}
	}()
	var readErr error
	var rwg sync.WaitGroup
	rwg.Add(1)
	go func() {
		defer rwg.Done()
		defer close(readCh)
		for block := range in {
			if block.Error != nil {
				readErr = block.Error
			}
			readCh <- workload{
				pj:  block.Value,
				dst: make(chan []byte, 0),
			}
		}
	}()
	rwg.Wait()
	if readErr != nil {
		wg.Wait()
		close(writeCh)
		wwg.Wait()
		return readErr
	}
	// Read done, wait for workers...
	wg.Wait()
	close(writeCh)
	// Wait for writer...
	wwg.Wait()
	return writeErr
}

const (
	tagFloatWithFlag = Tag('e')
)

// Serialize the data in pj and return the data.
// An optional destination can be provided.
func (s *Serializer) Serialize(dst []byte, pj ParsedJson) []byte {
	// Blocks:
	//  - Compressed size of entire block following. Can be 0 if empty. (varuint)
	//  - Block type, byte:
	//     0: uncompressed, rest is data.
	// 	   1: S2 compressed stream.
	// 	   2: Zstd block.
	//  - Compressed data.
	//
	// Serialized format:
	// - Header: Version (byte)
	// - Compressed size of remaining data (varuint). Excludes previous and size of this.
	// - Tape size, uncompressed (varuint)
	// - Strings size, uncompressed (varuint)
	// - Strings Block: Compressed block. See above.
	// - Message size, uncompressed (varuint)
	// - Message Block: Compressed block. See above.
	// - Uncompressed size of tags (varuint)
	// - Tags Block: Compressed block. See above.
	// - Uncompressed values size (varuint)
	// - Values Block: Compressed block. See above.
	//
	// Reconstruction:
	//
	// Read next tag. Depending on the tag, read a number of values:
	// Values:
	// 	 - Null, BoolTrue/BoolFalse: No value.
	//   - TagObjectStart, TagArrayStart, TagRoot: (Offset - Current offset). Write end tag for object and array.
	//   - TagObjectEnd, TagArrayEnd: No value stored, derived from start.
	//   - TagInteger, TagUint, TagFloat: 64 bits
	// 	 - TagString: offset, length stored.
	//   - tagFloatWithFlag (v2): Contains float parsing flag.
	//
	// If there are any values left as tag or value, it is considered invalid.

	var wg sync.WaitGroup

	// Reset lookup table.
	// Offsets are offset by 1, so 0 indicates an unfilled entry.
	for i := range s.stringsTable[:] {
		s.stringsTable[i] = 0
	}
	if len(s.stringBuf) > 0 {
		s.stringBuf = s.stringBuf[:0]
	}
	if len(s.sMsg) > 0 {
		s.sMsg = s.sMsg[:0]
	}

	msgWr, msgDone := encBlock(s.compStrings, s.sMsg, s.fasterComp)
	s.stringWr = msgWr

	const tagBufSize = 64 << 10
	const valBufSize = 64 << 10

	valWr, valDone := encBlock(s.compValues, s.valuesCompBuf, s.fasterComp)
	tagWr, tagDone := encBlock(s.compTags, s.tagsCompBuf, s.fasterComp)
	// Pessimistically allocate for maximum possible size.
	if cap(s.tagsBuf) <= tagBufSize {
		s.tagsBuf = make([]byte, tagBufSize)
	}
	s.tagsBuf = s.tagsBuf[:tagBufSize]

	// At most one value per 2 tape entries
	if cap(s.valuesBuf) < valBufSize+4 {
		s.valuesBuf = make([]byte, valBufSize+4)
	}

	s.valuesBuf = s.valuesBuf[:0]
	off := 0
	tagsOff := 0
	var tmp [8]byte
	rawValues := 0
	rawTags := 0
	for off < len(pj.Tape) {
		if tagsOff >= tagBufSize {
			rawTags += tagsOff
			tagWr.Write(s.tagsBuf[:tagsOff])
			tagsOff = 0
		}
		if len(s.valuesBuf) >= valBufSize {
			rawValues += len(s.valuesBuf)
			valWr.Write(s.valuesBuf)
			s.valuesBuf = s.valuesBuf[:0]
		}
		entry := pj.Tape[off]
		ntype := Tag(entry >> 56)
		payload := entry & JSONVALUEMASK

		switch ntype {
		case TagString:
			sb, err := pj.stringByteAt(payload, pj.Tape[off+1])
			if err != nil {
				panic(err)
			}
			offset := s.indexString(sb)

			binary.LittleEndian.PutUint64(tmp[:], offset)
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			binary.LittleEndian.PutUint64(tmp[:], uint64(len(sb)))
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			off++
		case TagUint:
			binary.LittleEndian.PutUint64(tmp[:], pj.Tape[off+1])
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			off++
		case TagInteger:
			binary.LittleEndian.PutUint64(tmp[:], pj.Tape[off+1])
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			off++
		case TagFloat:
			if payload == 0 {
				binary.LittleEndian.PutUint64(tmp[:], pj.Tape[off+1])
				s.valuesBuf = append(s.valuesBuf, tmp[:]...)
				off++
			} else {
				ntype = tagFloatWithFlag
				binary.LittleEndian.PutUint64(tmp[:], entry)
				s.valuesBuf = append(s.valuesBuf, tmp[:]...)
				binary.LittleEndian.PutUint64(tmp[:], pj.Tape[off+1])
				s.valuesBuf = append(s.valuesBuf, tmp[:]...)
				off++
			}
		case TagNull, TagBoolTrue, TagBoolFalse:
			// No value.
		case TagObjectStart, TagArrayStart, TagRoot:
			// TagObjectStart TagArrayStart always points forward.
			// TagRoot can point either direction so we rely on under/overflow.
			binary.LittleEndian.PutUint64(tmp[:], payload-uint64(off))
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
		case TagObjectEnd, TagArrayEnd, TagEnd:
			// Value can be deducted from start tag or no value.
		default:
			wg.Wait()
			panic(fmt.Errorf("unknown tag: %d", int(ntype)))
		}
		s.tagsBuf[tagsOff] = uint8(ntype)
		tagsOff++
		off++
	}
	if tagsOff > 0 {
		rawTags += tagsOff
		tagWr.Write(s.tagsBuf[:tagsOff])
	}
	if len(s.valuesBuf) > 0 {
		rawValues += len(s.valuesBuf)
		valWr.Write(s.valuesBuf)
	}
	wg.Add(3)
	go func() {
		var err error
		s.tagsCompBuf, err = tagDone()
		if err != nil {
			panic(err)
		}
		wg.Done()
	}()
	go func() {
		var err error
		s.valuesCompBuf, err = valDone()
		if err != nil {
			panic(err)
		}
		wg.Done()
	}()
	go func() {
		var err error
		s.sMsg, err = msgDone()
		if err != nil {
			panic(err)
		}
		wg.Done()
	}()

	// Wait for compressors
	wg.Wait()

	// Version
	dst = append(dst, serializedVersion)

	// Size of varints...
	varInts := binary.PutUvarint(tmp[:], uint64(0)) +
		binary.PutUvarint(tmp[:], uint64(len(s.sMsg))) +
		binary.PutUvarint(tmp[:], uint64(rawTags)) +
		binary.PutUvarint(tmp[:], uint64(len(s.tagsCompBuf))) +
		binary.PutUvarint(tmp[:], uint64(rawValues)) +
		binary.PutUvarint(tmp[:], uint64(len(s.valuesCompBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.stringBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(pj.Tape)))

	n := binary.PutUvarint(tmp[:], uint64(1+len(s.sMsg)+len(s.tagsCompBuf)+len(s.valuesCompBuf)+varInts))
	dst = append(dst, tmp[:n]...)

	// Tape elements, uncompressed.
	n = binary.PutUvarint(tmp[:], uint64(len(pj.Tape)))
	dst = append(dst, tmp[:n]...)

	// Strings uncompressed size
	dst = append(dst, 0)
	// Strings
	dst = append(dst, 0)

	// Messages uncompressed size
	n = binary.PutUvarint(tmp[:], uint64(len(s.stringBuf)))
	dst = append(dst, tmp[:n]...)
	// Message
	n = binary.PutUvarint(tmp[:], uint64(len(s.sMsg)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.sMsg...)

	// Tags
	n = binary.PutUvarint(tmp[:], uint64(rawTags))
	dst = append(dst, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(len(s.tagsCompBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.tagsCompBuf...)

	// Values
	n = binary.PutUvarint(tmp[:], uint64(rawValues))
	dst = append(dst, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(len(s.valuesCompBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.valuesCompBuf...)
	if false {
		fmt.Println("strings:", len(pj.Strings)+len(pj.Message), "->", len(s.sMsg), "tags:", rawTags, "->", len(s.tagsCompBuf), "values:", rawValues, "->", len(s.valuesCompBuf), "Total:", len(pj.Message)+len(pj.Strings)+len(pj.Tape)*8, "->", len(dst))
	}

	return dst
}

func (s *Serializer) splitBlocks(r io.Reader, out chan []byte) error {
	br := bufio.NewReader(r)
	defer close(out)
	for {
		if v, err := br.ReadByte(); err != nil {
			return err
		} else if v != 1 {
			return errors.New("unknown version")
		}

		// Comp size
		c, err := binary.ReadUvarint(br)
		if err != nil {
			return err
		}
		if c > s.maxBlockSize {
			return errors.New("compressed block too big")
		}
		block := make([]byte, c)
		n, err := io.ReadFull(br, block)
		if err != nil {
			return err
		}
		if n > 0 {
			out <- block
		}
	}
}

// Deserialize the content in src.
// Only basic sanity checks will be performed.
// Slight corruption will likely go through unnoticed.
// And optional destination can be provided.
func (s *Serializer) Deserialize(src []byte, dst *ParsedJson) (*ParsedJson, error) {
	br := bytes.NewBuffer(src)

	if v, err := br.ReadByte(); err != nil {
		return dst, err
	} else if v > serializedVersion {
		// v2 reads v1.
		return dst, errors.New("unknown version")
	}

	if dst == nil {
		dst = &ParsedJson{}
	}

	// Comp size
	if c, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if int(c) > br.Len() {
			return dst, fmt.Errorf("stream too short, want %d, only have %d left", c, br.Len())
		}
		if int(c) > br.Len() {
			fmt.Println("extra length:", int(c), br.Len())
		}
	}

	// Tape size
	if ts, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(dst.Tape)) < ts {
			dst.Tape = make([]uint64, ts)
		}
		dst.Tape = dst.Tape[:ts]
	}

	// String size
	if ss, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(dst.Strings)) < ss || dst.Strings == nil {
			dst.Strings = make([]byte, ss)
		}
		dst.Strings = dst.Strings[:ss]
	}

	// Decompress strings
	var sWG sync.WaitGroup
	var stringsErr, msgErr error
	err := s.decBlock(br, dst.Strings, &sWG, &stringsErr)
	if err != nil {
		return dst, err
	}

	// Message size
	if ss, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(dst.Message)) < ss || dst.Message == nil {
			dst.Message = make([]byte, ss)
		}
		dst.Message = dst.Message[:ss]
	}

	// Messages
	err = s.decBlock(br, dst.Message, &sWG, &msgErr)
	if err != nil {
		return dst, err
	}
	defer sWG.Wait()

	// Decompress tags
	if tags, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(s.tagsBuf)) < tags {
			s.tagsBuf = make([]byte, tags)
		}
		s.tagsBuf = s.tagsBuf[:tags]
	}

	var wg sync.WaitGroup
	var tagsErr error
	err = s.decBlock(br, s.tagsBuf, &wg, &tagsErr)
	if err != nil {
		return dst, fmt.Errorf("decompressing tags: %w", err)
	}
	defer wg.Wait()

	// Decompress values
	if vals, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(s.valuesBuf)) < vals {
			s.valuesBuf = make([]byte, vals)
		}
		s.valuesBuf = s.valuesBuf[:vals]
	}

	var valsErr error
	err = s.decBlock(br, s.valuesBuf, &wg, &valsErr)
	if err != nil {
		return dst, fmt.Errorf("decompressing values: %w", err)
	}

	// Wait until we have what we need for the tape.
	wg.Wait()
	switch {
	case tagsErr != nil:
		return dst, fmt.Errorf("decompressing tags: %w", tagsErr)
	case valsErr != nil:
		return dst, fmt.Errorf("decompressing values: %w", valsErr)
	}

	// Reconstruct tape:
	var off int
	values := s.valuesBuf
	for _, t := range s.tagsBuf {
		if off == len(dst.Tape) {
			return dst, errors.New("tags extended beyond tape")
		}
		tag := Tag(t)

		tagDst := uint64(t) << 56
		switch tag {
		case TagString:
			if len(values) < 16 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			sOffset := binary.LittleEndian.Uint64(values[:8])
			sLen := binary.LittleEndian.Uint64(values[8:16])
			values = values[16:]

			dst.Tape[off] = tagDst | sOffset
			dst.Tape[off+1] = sLen
			off += 2
		case TagFloat, TagInteger, TagUint:
			if len(values) < 8 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			dst.Tape[off] = tagDst
			dst.Tape[off+1] = binary.LittleEndian.Uint64(values[:8])
			values = values[8:]
			off += 2
		case tagFloatWithFlag:
			// Tape contains full value
			if len(values) < 16 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			dst.Tape[off] = binary.LittleEndian.Uint64(values[:8])
			dst.Tape[off+1] = binary.LittleEndian.Uint64(values[8:16])
			values = values[16:]
			off += 2
		case TagNull, TagBoolTrue, TagBoolFalse, TagEnd:
			dst.Tape[off] = tagDst
			off++
		case TagObjectStart, TagArrayStart:
			if len(values) < 8 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			// Always forward
			val := binary.LittleEndian.Uint64(values[:8])
			values = values[8:]
			val += uint64(off)
			if val > uint64(len(dst.Tape)) {
				return dst, fmt.Errorf("%v extends beyond tape (%d). offset:%d", tag, len(dst.Tape), val)
			}

			dst.Tape[off] = tagDst | val
			// Write closing...
			dst.Tape[val-1] = uint64(tagOpenToClose[tag])<<56 | uint64(off)

			off++
		case TagRoot:
			if len(values) < 8 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			// Always forward
			val := binary.LittleEndian.Uint64(values[:8])
			values = values[8:]
			val += uint64(off)
			if val > uint64(len(dst.Tape)) {
				return dst, fmt.Errorf("%v extends beyond tape (%d). offset:%d", tag, len(dst.Tape), val)
			}

			dst.Tape[off] = tagDst | val

			off++
		case TagObjectEnd, TagArrayEnd:
			// This should already have been written.
			if dst.Tape[off]&JSONTAGMASK != tagDst {
				return dst, fmt.Errorf("reading %v, offset:%d, start tag did not match %x != %x", tag, off, dst.Tape[off]>>56, uint8(tag))
			}
			off++
		default:
			return nil, fmt.Errorf("unknown tag: %v", tag)
		}
	}
	sWG.Wait()
	if off != len(dst.Tape) {
		return dst, fmt.Errorf("tags did not fill tape, want %d, got %d", len(dst.Tape), off)
	}
	if len(values) > 0 {
		return dst, fmt.Errorf("values did not fill tape, want %d, got %d", len(dst.Tape), off)
	}
	if stringsErr != nil {
		return dst, fmt.Errorf("reading strings: %w", stringsErr)
	}
	return dst, nil
}

func (s *Serializer) decBlock(br *bytes.Buffer, dst []byte, wg *sync.WaitGroup, dstErr *error) error {
	size, err := binary.ReadUvarint(br)
	if err != nil {
		return err
	}
	if size > uint64(br.Len()) {
		return fmt.Errorf("block size (%d) extends beyond input %d", size, br.Len())
	}
	if size == 0 && len(dst) == 0 {
		// Nothing, no compress type
		return nil
	}
	if size < 1 {
		return fmt.Errorf("block size (%d) too small %d", size, br.Len())
	}

	typ, err := br.ReadByte()
	if err != nil {
		return err
	}
	size--
	compressed := br.Next(int(size))
	if len(compressed) != int(size) {
		return errors.New("short block section")
	}
	switch typ {
	case blockTypeUncompressed:
		// uncompressed
		if len(compressed) != len(dst) {
			panic("err")
			return fmt.Errorf("short uncompressed block: in (%d) != out (%d)", len(compressed), len(dst))
		}
		copy(dst, compressed)
	case blockTypeS2:
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := bytes.NewBuffer(compressed)
			dec := s2Readers.Get().(*s2.Reader)
			dec.Reset(buf)
			_, err := io.ReadFull(dec, dst)
			dec.Reset(nil)
			s2Readers.Put(dec)
			*dstErr = err
		}()
	case blockTypeZstd:
		wg.Add(1)
		go func() {
			defer wg.Done()
			want := len(dst)
			dst, err = zDec.DecodeAll(compressed, dst[:0])
			if err == nil && want != len(dst) {
				err = errors.New("zstd decompressed size mismatch")
			}
			*dstErr = err
		}()
	default:
		return fmt.Errorf("unknown compression type: %d", typ)
	}
	return nil
}

const (
	blockTypeUncompressed byte = 0
	blockTypeS2           byte = 1
	blockTypeZstd         byte = 2
)

var zDec *zstd.Decoder

var zEncFast = sync.Pool{New: func() interface{} {
	e, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderCRC(false))
	return e
}}

var s2FastWriters = sync.Pool{New: func() interface{} {
	return s2.NewWriter(nil)
}}

var s2Writers = sync.Pool{New: func() interface{} {
	return s2.NewWriter(nil, s2.WriterBetterCompression())
}}
var s2Readers = sync.Pool{New: func() interface{} {
	return s2.NewReader(nil)
}}

var initSerializerOnce sync.Once

func initSerializer() {
	zDec, _ = zstd.NewReader(nil)
}

type encodedResult func() ([]byte, error)

// encBlock will encode a block of data.
func encBlock(mode byte, buf []byte, fast bool) (io.Writer, encodedResult) {
	dst := bytes.NewBuffer(buf[:0])
	dst.WriteByte(mode)
	switch mode {
	case blockTypeUncompressed:
		return dst, func() ([]byte, error) {
			return dst.Bytes(), nil
		}
	case blockTypeS2:
		var enc *s2.Writer
		var put *sync.Pool
		if fast {
			enc = s2FastWriters.Get().(*s2.Writer)
			put = &s2FastWriters
		} else {
			enc = s2Writers.Get().(*s2.Writer)
			put = &s2Writers
		}
		enc.Reset(dst)
		return enc, func() (i []byte, err error) {
			err = enc.Close()
			if err != nil {
				return nil, err
			}
			enc.Reset(nil)
			put.Put(enc)
			return dst.Bytes(), nil
		}
	case blockTypeZstd:
		enc := zEncFast.Get().(*zstd.Encoder)
		enc.Reset(dst)
		return enc, func() (i []byte, err error) {
			err = enc.Close()
			if err != nil {
				return nil, err
			}
			enc.Reset(nil)
			zEncFast.Put(enc)
			return dst.Bytes(), nil
		}
	}
	panic("unknown compression mode")
}

// indexString will deduplicate strings and populate
func (s *Serializer) indexString(sb []byte) (offset uint64) {
	// Only possible on 64 bit platforms, so it will never trigger on 32 bit platforms.
	if uint32(len(sb)) >= math.MaxUint32 {
		panic("string too long")
	}

	h := memHash(sb) & stringmask
	off := int(s.stringsTable[h]) - 1
	end := off + len(sb)
	if off >= 0 && end <= len(s.stringBuf) {
		found := s.stringBuf[off:end]
		if bytes.Equal(found, sb) {
			return uint64(off)
		}
		// It didn't match :(
	}
	off = len(s.stringBuf)
	s.stringBuf = append(s.stringBuf, sb...)
	s.stringsTable[h] = uint32(off + 1)
	s.stringWr.Write(sb)
	return uint64(off)
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr

// memHash is the hash function used by go map, it utilizes available hardware instructions (behaves
// as aeshash if aes instruction is available).
// NOTE: The hash seed changes for every process. So, this cannot be used as a persistent hash.
func memHash(data []byte) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&data))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

type stringStruct struct {
	str unsafe.Pointer
	len int
}
