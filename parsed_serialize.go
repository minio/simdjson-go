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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
	"io"
	"sync"
)

// Serializer allows to serialize parsed json and read it back.
// A Serializer can be reused, but not used concurrently.
type Serializer struct {
	stringBuf []byte

	// Compressed strings
	sBuf []byte
	sMsg []byte
	// Uncompressed tags
	tagsBuf []byte
	// Values
	valuesBuf     []byte
	valuesCompBuf []byte
	tagsCompBuf   []byte

	compValues, compTags uint8
	compStrings          bool
	alwaysZstdStrings    bool
	neverZstdStrings     bool
}

// NewSerializer will create and initialize a serializer.
func NewSerializer() *Serializer {
	initSerializerOnce.Do(initSerializer)
	var s Serializer
	s.CompressMode(CompressDefault)
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
		s.compStrings = false
	case CompressFast:
		s.compValues = blockTypeS2
		s.compTags = blockTypeS2
		s.compStrings = true
		s.alwaysZstdStrings = false
		s.neverZstdStrings = true
	case CompressDefault:
		s.compValues = blockTypeS2
		s.compTags = blockTypeS2
		s.compStrings = true
		s.alwaysZstdStrings = false
	case CompressBest:
		s.compValues = blockTypeZstd
		s.compTags = blockTypeZstd
		s.compStrings = true
		s.alwaysZstdStrings = true
	default:
		panic("unknown compression mode")
	}
}

// Serialize the data in pj and return the data.
// An optional destination can be provided.
func (s *Serializer) Serialize(dst []byte, pj ParsedJson) []byte {
	// Header: Version byte
	// Varuint Strings size, uncompressed
	// Varuint Tape size, uncompressed
	// Varuint Compressed size of remaining data.
	// Strings:
	// - Varint: Compressed bytes total.
	//  Compressed Blocks:
	//     - Varint: Block compressed bytes excluding this varint.
	//     - Block type:
	//     		0: uncompressed, rest is data.
	// 			1: S2 block.
	// 			2: Zstd block.
	// 	   - block data.
	// 	   Ends when total is reached.
	// Varuint: Tape length, Unique Elements
	// Tags: Compressed block
	// - Byte type:
	// 		- 0: Uncompressed.
	//      - 3: zstd compressed block.
	// 		- 4: S2 compressed block.
	// - Varuint compressed size.
	// - Table + data.
	// Values:
	// - Varint total compressed size.
	//  S2 block.
	// 	 - Null, BoolTrue/BoolFalse: Nothing added.
	//   - TagObjectStart, TagArrayStart, TagRoot: Offset - Current offset
	//   - TagObjectEnd, TagArrayEnd: Current offset - Offset
	//   - TagInteger, TagUint, TagFloat: 64 bits
	// 	 - TagString: offset

	// Index strings
	s.stringBuf = pj.Strings
	var wg sync.WaitGroup

	if s.compStrings {
		// Choose zstd when tape is likely to take longer than strings.
		zstdStrings := len(s.stringBuf) < len(pj.Tape)*15 && !s.neverZstdStrings
		wg.Add(2)
		go func() {
			defer wg.Done()
			if zstdStrings || s.alwaysZstdStrings {
				s.sBuf = encBlock(blockTypeZstd, s.stringBuf, s.sBuf)
			} else {
				s.sBuf = encBlock(blockTypeS2, s.stringBuf, s.sBuf)
			}
		}()
		zstdMsg := len(pj.Message) < len(pj.Tape)*15 && !s.neverZstdStrings
		go func() {
			defer wg.Done()
			if zstdMsg || s.alwaysZstdStrings {
				s.sMsg = encBlock(blockTypeZstd, pj.Message, s.sMsg)
			} else {
				s.sMsg = encBlock(blockTypeS2, pj.Message, s.sMsg)
			}
		}()
	} else {
		s.sBuf = encBlock(blockTypeUncompressed, s.stringBuf, s.sBuf)
		s.sMsg = encBlock(blockTypeUncompressed, pj.Message, s.sMsg)
	}

	// Pessimistically allocate for maximum possible size.
	if cap(s.tagsBuf) <= len(pj.Tape) {
		s.tagsBuf = make([]byte, len(pj.Tape)+1)
	}
	s.tagsBuf = s.tagsBuf[:len(pj.Tape)+1]

	// At most one value per 2 tape entries
	if cap(s.valuesBuf) < len(pj.Tape)*4 {
		s.valuesBuf = make([]byte, len(pj.Tape)*4)
	}
	s.valuesBuf = s.valuesBuf[:0]
	off := 0
	tagsOff := 0
	var tmp [8]byte
	for off < len(pj.Tape) {
		entry := pj.Tape[off]
		ntype := Tag(entry >> 56)
		payload := entry & JSONVALUEMASK
		s.tagsBuf[tagsOff] = uint8(ntype)
		tagsOff++

		switch ntype {
		case TagString:
			binary.LittleEndian.PutUint64(tmp[:], payload)
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			binary.LittleEndian.PutUint64(tmp[:8], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			off++
		case TagUint:
			binary.LittleEndian.PutUint64(tmp[:], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			off++
		case TagInteger:
			binary.LittleEndian.PutUint64(tmp[:], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
			off++
		case TagFloat:
			binary.LittleEndian.PutUint64(tmp[:8], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:8]...)
			off++
		case TagNull, TagBoolTrue, TagBoolFalse:
			// No value.
		case TagObjectStart, TagArrayStart, TagRoot:
			// Always forward
			binary.LittleEndian.PutUint64(tmp[:], payload-uint64(off))
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
		case TagObjectEnd, TagArrayEnd:
			// Always backward
			binary.LittleEndian.PutUint64(tmp[:], uint64(off)-payload)
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
		case TagEnd:
			// Nothing to store
		default:
			wg.Wait()
			panic(fmt.Errorf("unknown tag: %d", int(ntype)))
		}
		off++
	}
	wg.Add(2)
	// Compress values
	go func() {
		defer wg.Done()
		s.valuesCompBuf = encBlock(s.compValues, s.valuesBuf, s.valuesCompBuf)
	}()

	// Compress tags
	s.tagsBuf = s.tagsBuf[:tagsOff]
	go func() {
		defer wg.Done()
		s.tagsCompBuf = encBlock(s.compTags, s.tagsBuf, s.tagsCompBuf)
	}()

	// Wait for compressors
	wg.Wait()

	// Version
	dst = append(dst, 1)
	// Strings uncompressed size
	n := binary.PutUvarint(tmp[:], uint64(len(s.stringBuf)))
	dst = append(dst, tmp[:n]...)
	// Strings uncompressed size
	n = binary.PutUvarint(tmp[:], uint64(len(pj.Message)))
	dst = append(dst, tmp[:n]...)
	// Tape elements, uncompressed.
	n = binary.PutUvarint(tmp[:], uint64(len(pj.Tape)))
	dst = append(dst, tmp[:n]...)

	// Size of varints...
	varInts := binary.PutUvarint(tmp[:], uint64(len(s.sBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.sMsg))) +
		binary.PutUvarint(tmp[:], uint64(len(s.tagsBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.tagsCompBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.valuesBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.valuesCompBuf)))

	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)+len(s.sMsg)+len(s.tagsCompBuf)+len(s.valuesCompBuf)+varInts))
	dst = append(dst, tmp[:n]...)

	// Strings
	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.sBuf...)

	// Message
	n = binary.PutUvarint(tmp[:], uint64(len(s.sMsg)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.sMsg...)

	// Tags
	n = binary.PutUvarint(tmp[:], uint64(len(s.tagsBuf)))
	dst = append(dst, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(len(s.tagsCompBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.tagsCompBuf...)

	// Values
	n = binary.PutUvarint(tmp[:], uint64(len(s.valuesBuf)))
	dst = append(dst, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(len(s.valuesCompBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.valuesCompBuf...)
	if false {
		fmt.Println("strings:", len(pj.Strings), "->", len(s.sBuf), "messages:", len(pj.Message), "->", len(s.sMsg), "tags:", len(s.tagsBuf), "->", len(s.tagsCompBuf), "values:", len(s.valuesBuf), "->", len(s.valuesCompBuf), "Total:", len(pj.Strings)+len(pj.Tape)*8, "->", len(dst))
	}

	return dst
}

// Deserialize the content in src.
// Only basic sanity checks will be performed.
// Slight corruption will likely go through unnoticed.
// And optional destination can be provided.
func (s *Serializer) Deserialize(src []byte, dst *ParsedJson) (*ParsedJson, error) {
	br := bytes.NewBuffer(src)

	if v, err := br.ReadByte(); err != nil {
		return dst, err
	} else if v != 1 {
		return dst, errors.New("unknown version")
	}

	if dst == nil {
		dst = &ParsedJson{}
	}
	// String size
	if ss, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(dst.Strings)) < ss {
			dst.Strings = make([]byte, ss)
		}
		dst.Strings = dst.Strings[:ss]
	}
	// Message size
	if ss, err := binary.ReadUvarint(br); err != nil {
		return dst, err
	} else {
		if uint64(cap(dst.Message)) < ss {
			dst.Message = make([]byte, ss)
		}
		dst.Message = dst.Message[:ss]
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

	// Decompress strings
	var sWG sync.WaitGroup
	var stringsErr, msgErr error
	err := s.decBlock(br, dst.Strings, &sWG, &stringsErr)
	if err != nil {
		return dst, err
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
			if false && sOffset+sLen > uint64(len(dst.Strings)) {
				// TODO: Maybe validate
				return dst, fmt.Errorf("%v extends beyond stringbuf (%d). offset:%d", tag, len(dst.Strings), sOffset)
			}

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
		case TagNull, TagBoolTrue, TagBoolFalse:
			dst.Tape[off] = tagDst
			off++
		case TagObjectStart, TagArrayStart, TagRoot:
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
			if len(values) < 8 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			// Always backward
			val := binary.LittleEndian.Uint64(values[:8])
			values = values[8:]
			val = uint64(off) - val
			if val > uint64(len(dst.Tape)) {
				return dst, fmt.Errorf("%v extends beyond tape (%d). offset:%d", tag, len(dst.Tape), val)
			}
			dst.Tape[off] = tagDst | val
			off++
		case TagEnd:
			dst.Tape[off] = tagDst
			off++
		default:
			return nil, fmt.Errorf("unknown tag: %v", tag)
		}
	}
	if off != len(dst.Tape) {
		return dst, fmt.Errorf("tags did not fill tape, want %d, got %d", len(dst.Tape), off)
	}
	sWG.Wait()
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
			return errors.New("short uncompressed block")
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
var zEncFast *zstd.Encoder
var s2Writers = sync.Pool{New: func() interface{} {
	return s2.NewWriter(nil)
}}
var s2Readers = sync.Pool{New: func() interface{} {
	return s2.NewReader(nil)
}}

var initSerializerOnce sync.Once

func initSerializer() {
	zDec, _ = zstd.NewReader(nil)
	zEncFast, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderCRC(false))
}

// encBlock will encode a block of data.
func encBlock(mode byte, src, dst []byte) []byte {
	if len(src) < 100 {
		mode = blockTypeUncompressed
	}
	switch mode {
	case blockTypeUncompressed:
		mel := len(src) + 1
		if cap(dst) < mel {
			dst = make([]byte, mel)
		}
		dst = dst[:mel]
		dst[0] = mode
		copy(dst[1:], src)
		return dst
	case blockTypeS2:
		mel := s2.MaxEncodedLen(len(src)) + 16
		if cap(dst) < mel {
			dst = make([]byte, mel)
		}
		dst = dst[:1]
		dst[0] = mode
		buf := bytes.NewBuffer(dst[:1])
		enc := s2Writers.Get().(*s2.Writer)
		enc.Reset(buf)
		err := enc.EncodeBuffer(src)
		if err != nil {
			panic(err)
		}
		err = enc.Close()
		if err != nil {
			panic(err)
		}
		enc.Reset(nil)
		s2Writers.Put(enc)
		return buf.Bytes()
	case blockTypeZstd:
		mel := len(src) + 50
		if cap(dst) < mel {
			dst = make([]byte, mel)
		}
		dst = dst[:mel]
		dst[0] = mode
		return zEncFast.EncodeAll(src, dst[:1])
	}
	panic("unknown compression mode")
}
