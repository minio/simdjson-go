package simdjson

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sync"
	"unsafe"

	"github.com/klauspost/compress/fse"
	"github.com/klauspost/compress/huff0"
	"github.com/klauspost/compress/s2"
	"github.com/klauspost/compress/zstd"
)

const (
	stringBits = 14
	stringSize = 1 << stringBits
	stringmask = stringSize - 1
)

type serializer struct {
	tComp   fse.Scratch
	sComp   huff0.Scratch
	strings map[string]uint32
	// Old -> new offset
	stringIdxLUT []uint32
	stringBuf    []byte

	// Compressed strings
	sBuf []byte
	// Uncompressed tags
	tagsBuf []byte
	// Values
	valuesBuf     []byte
	valuesCompBuf []byte
	tagsCompBuf   []byte
	strings2      [stringSize]uint32
}

func (s *serializer) Serialize2(dst []byte, pj ParsedJson) ([]byte, error) {
	dst = zEncFast.EncodeAll(pj.Strings, dst)
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&pj.Tape))
	header.Len *= 8
	header.Cap *= 8

	data := *(*[]byte)(unsafe.Pointer(&header))
	dst = zEncFast.EncodeAll(data, dst)
	return dst, nil
}

func (s *serializer) Serialize3(dst []byte, pj ParsedJson) ([]byte, error) {
	dst = append(dst, pj.Strings...)
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&pj.Tape))
	header.Len *= 8
	header.Cap *= 8

	data := *(*[]byte)(unsafe.Pointer(&header))
	dst = append(dst, data...)
	return dst, nil
}

func (s *serializer) Serialize4(dst []byte, pj ParsedJson) ([]byte, error) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mel := s2.MaxEncodedLen(len(pj.Strings))
		if cap(s.valuesBuf) < mel {
			s.valuesBuf = make([]byte, 0, mel)
		}
		s.valuesBuf = s.valuesBuf[:mel]
		s.valuesBuf = s2.Encode(s.valuesBuf, pj.Strings)
	}()
	go func() {
		defer wg.Done()
		header := *(*reflect.SliceHeader)(unsafe.Pointer(&pj.Tape))
		header.Len *= 8
		header.Cap *= 8

		data := *(*[]byte)(unsafe.Pointer(&header))
		mel := s2.MaxEncodedLen(len(data))
		if cap(s.sBuf) < mel {
			s.sBuf = make([]byte, 0, mel)
		}
		s.sBuf = s.sBuf[:mel]
		s.sBuf = s2.Encode(s.sBuf, data)
	}()
	wg.Wait()
	dst = append(dst, s.valuesBuf...)
	dst = append(dst, s.sBuf...)
	return dst, nil
}

func (s *serializer) Serialize(dst []byte, pj ParsedJson) ([]byte, error) {
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
	//   - TagRoot:Absolute Varuint offset.
	//   - TagObjectStart, TagArrayStart: varuint: Offset - Current offset
	//   - TagObjectEnd, TagArrayEnd: varuint: Current offset - Offset
	//   - TagInteger: Varint
	//   - TagUint: Varuint
	//   - TagFloat: 64 bits
	// 	 - TagString: Varuint offset

	const reIndexStrings = true
	// Index strings
	if reIndexStrings {
		err := s.indexStringsLazy(pj.Strings)
		if err != nil {
			return nil, err
		}
	} else {
		s.stringBuf = pj.Strings
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var compErr error
	// Choose zstd when tape is likely to take longer than strings.
	zstdStrings := len(s.stringBuf) < len(pj.Tape)*10
	go func() {
		defer wg.Done()
		if zstdStrings {
			compErr = s.compressStringsZstd()
		} else {
			compErr = s.compressStringsS2()
		}
	}()
	//fmt.Println("strings dedupe:", len(pj.Strings), "->", len(s.stringBuf))

	// Pessimistically allocate for maximum possible size.
	if cap(s.tagsBuf) <= len(pj.Tape) {
		s.tagsBuf = make([]byte, len(pj.Tape)+1)
	}
	s.tagsBuf = s.tagsBuf[:len(pj.Tape)+1]
	if cap(s.valuesBuf) < len(pj.Tape)*binary.MaxVarintLen64 {
		s.valuesBuf = make([]byte, len(pj.Tape)*binary.MaxVarintLen64)
	}
	s.valuesBuf = s.valuesBuf[:0]
	off := 0
	tagsOff := 0
	var tmp [binary.MaxVarintLen64]byte
	for off < len(pj.Tape) {
		entry := pj.Tape[off]
		ntype := Tag(entry >> 56)
		payload := entry & JSONVALUEMASK
		s.tagsBuf[tagsOff] = uint8(ntype)
		tagsOff++

		switch ntype {
		case TagString:
			var sOffset uint32
			if reIndexStrings {
				sOffset = s.stringIdxLUT[uint32(payload)/4]
			} else {
				sOffset = uint32(payload)
			}
			n := binary.PutUvarint(tmp[:], uint64(sOffset))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagUint:
			n := binary.PutUvarint(tmp[:], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
			off++
		case TagInteger:
			n := binary.PutVarint(tmp[:], int64(pj.Tape[off]))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
			off++
		case TagFloat:
			binary.LittleEndian.PutUint64(tmp[:8], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:8]...)
			off++
		case TagNull, TagBoolTrue, TagBoolFalse:
			// No value.
		case TagObjectStart, TagArrayStart:
			// Always forward
			n := binary.PutUvarint(tmp[:], payload-uint64(off))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagObjectEnd, TagArrayEnd:
			// Always backward
			n := binary.PutUvarint(tmp[:], uint64(off)-payload)
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagRoot:
			// We cannot detect direction, so we encode as signed offset.
			n := binary.PutVarint(tmp[:], int64(payload)-int64(off))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		default:
			wg.Wait()
			return nil, fmt.Errorf("unknown tag: %v", ntype)
		}
		off++
	}
	wg.Add(1)
	// Compress values
	const valuesZstd = false
	valueCompType := blockTypeUncompressed
	var compValues []byte
	go func() {
		defer wg.Done()
		if valuesZstd {
			s.valuesCompBuf = zEncNoEnt.EncodeAll(s.valuesBuf, s.valuesCompBuf[:0])
			valueCompType = blockTypeZstd
		} else if true {
			mel := s2.MaxEncodedLen(len(s.valuesBuf))
			if cap(s.valuesCompBuf) < mel {
				s.valuesCompBuf = make([]byte, mel)
			}
			s.valuesCompBuf = s.valuesCompBuf[:mel]
			s.valuesCompBuf = s2.Encode(s.valuesCompBuf, s.valuesBuf)
			valueCompType = blockTypeS2
			compValues = s.valuesCompBuf
			if len(compValues) > len(s.valuesBuf) {
				compValues = s.valuesBuf
				valueCompType = blockTypeUncompressed // uncompressed
			}
		} else {
			compValues = s.valuesBuf
			valueCompType = blockTypeUncompressed // uncompressed
		}
	}()

	s.tagsBuf = s.tagsBuf[:tagsOff]
	var compTagsType byte
	var compTags []byte
	const zStdTags = false // s2 seems best
	if zStdTags {
		s.tagsCompBuf = zEncNoEnt.EncodeAll(s.tagsBuf, s.tagsCompBuf[:0])
		compTags = s.tagsCompBuf
		if len(compTags) > len(s.tagsBuf) {
			compTags = s.tagsBuf
			compTagsType = blockTypeUncompressed // uncompressed
		} else {
			compTagsType = blockTypeZstd // zstd
		}
	} else {
		mel := s2.MaxEncodedLen(len(s.tagsBuf))
		if cap(s.tagsCompBuf) < mel {
			s.tagsCompBuf = make([]byte, mel)
		}
		s.tagsCompBuf = s.tagsCompBuf[:mel]
		s.tagsCompBuf = s2.Encode(s.tagsCompBuf, s.tagsBuf)
		compTags = s.tagsCompBuf
		if len(compTags) > len(s.tagsBuf) {
			compTags = s.tagsBuf
			compTagsType = blockTypeUncompressed // uncompressed
		} else {
			compTagsType = blockTypeS2
		}
	}

	// Wait for compressors
	wg.Wait()
	if compErr != nil {
		return nil, compErr
	}

	// Version
	dst = append(dst, 1)
	// Strings uncompressed size
	n := binary.PutUvarint(tmp[:], uint64(len(s.stringBuf)))
	dst = append(dst, tmp[:n]...)
	// Tape elements, uncompressed.
	n = binary.PutUvarint(tmp[:], uint64(len(pj.Tape)))
	dst = append(dst, tmp[:n]...)

	// Size of varints...
	varInts := binary.PutUvarint(tmp[:], uint64(len(s.sBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.tagsBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(compTags))) +
		binary.PutUvarint(tmp[:], uint64(len(s.valuesBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(compValues)))

	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)+len(compTags)+len(compValues)+2+varInts))
	dst = append(dst, tmp[:n]...)

	// Strings
	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.sBuf...)

	// Tags
	n = binary.PutUvarint(tmp[:], uint64(len(s.tagsBuf)))
	dst = append(dst, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(len(compTags)+1))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, compTagsType)
	if len(compTags) > 0 {
		dst = append(dst, compTags...)
	}

	// Values
	n = binary.PutUvarint(tmp[:], uint64(len(s.valuesBuf)))
	dst = append(dst, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(len(compValues)+1))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, valueCompType)
	dst = append(dst, compValues...)
	if false {
		fmt.Println("strings:", len(pj.Strings), "->", len(s.sBuf), "tags:", len(s.tagsBuf), "->", len(compTags), "values:", len(s.valuesBuf), "->", len(compValues), "Total:", len(pj.Strings)+len(pj.Tape)*8, "->", len(dst))
	}

	return dst, nil
}

func (s *serializer) DeSerialize(src []byte, dst *ParsedJson) (*ParsedJson, error) {
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
	var stringsErr error
	err := s.decBlock(br, dst.Strings, &sWG, &stringsErr)
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
	values := bytes.NewBuffer(s.valuesBuf)
	var tmpBuf [8]byte
	for _, tag := range s.tagsBuf {
		if off == len(dst.Tape) {
			return dst, errors.New("tags extended beyond tape")
		}
		tagDst := uint64(tag) << 56
		switch Tag(tag) {
		case TagString:
			sOffset, err := binary.ReadUvarint(values)
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			if sOffset > uint64(len(dst.Strings)-5) {
				return dst, fmt.Errorf("%v extends beyond stringbuf (%d). offset:%d", tag, len(dst.Strings), sOffset)
			}

			dst.Tape[off] = tagDst | sOffset
			off++
		case TagUint:
			val, err := binary.ReadUvarint(values)
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			dst.Tape[off] = tagDst
			dst.Tape[off+1] = val
			off += 2
		case TagInteger:
			val, err := binary.ReadVarint(values)
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			dst.Tape[off] = tagDst
			dst.Tape[off+1] = uint64(val)
			off += 2
		case TagFloat:
			_, err := values.Read(tmpBuf[:])
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			val := binary.LittleEndian.Uint64(tmpBuf[:])
			dst.Tape[off] = tagDst
			dst.Tape[off+1] = val
			off += 2
		case TagNull, TagBoolTrue, TagBoolFalse:
			dst.Tape[off] = tagDst
			off++
		case TagObjectStart, TagArrayStart:
			// Always forward
			val, err := binary.ReadUvarint(values)
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			val += uint64(off)
			if val > uint64(len(dst.Tape)) {
				return dst, fmt.Errorf("%v extends beyond tape (%d). offset:%d", tag, len(dst.Tape), val)
			}

			dst.Tape[off] = tagDst | val
			off++
		case TagObjectEnd, TagArrayEnd:
			// Always backward
			val, err := binary.ReadUvarint(values)
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			val = uint64(off) - val
			if val > uint64(len(dst.Tape)) {
				return dst, fmt.Errorf("%v extends beyond tape (%d). offset:%d", tag, len(dst.Tape), val)
			}
			dst.Tape[off] = tagDst | val
			off++
		case TagRoot:
			// We cannot detect direction, so we encode as signed offset.
			val, err := binary.ReadVarint(values)
			if err != nil {
				return dst, fmt.Errorf("reading value: %w", err)
			}
			val2 := int64(off) + val
			if val2 < 0 {
				return dst, fmt.Errorf("root is negative. offset:%d, value read: %d", off, val)
			}
			if val2 > int64(len(dst.Tape)) {
				return dst, fmt.Errorf("root extends beyond tape (%d). offset:%d", len(dst.Tape), val2)
			}
			dst.Tape[off] = tagDst | uint64(val2)
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

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func (s *serializer) encBlockStream(mode byte, dst *bytes.Buffer) io.WriteCloser {
	switch mode {
	case blockTypeUncompressed:
		return nopWriteCloser{dst}
	case blockTypeZstd:
		zEncFast.Reset(dst)
		return zEncFast
	case blockTypeS2:
		return s2.NewWriter(dst)
	}
	panic("unknown compression")
}

func (s *serializer) decBlock(br *bytes.Buffer, dst []byte, wg *sync.WaitGroup, dstErr *error) error {
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
			want := len(dst)
			dst, err = s2.Decode(dst, compressed)
			if err == nil && want != len(dst) {
				err = errors.New("s2 decompressed size mismatch")
			}
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

type stream interface {
	io.Reader
	io.ByteReader
}

func (s *serializer) decBlockStream(br *bytes.Buffer, dst *bufio.Reader) (stream, error) {
	size, err := binary.ReadUvarint(br)
	if err != nil {
		return nil, err
	}
	if size > uint64(br.Len()) {
		return nil, fmt.Errorf("block size (%d) extends beyond input %d", size, br.Len())
	}
	if size < 1 {
		return nil, fmt.Errorf("block size (%d) too small %d", size, br.Len())
	}
	typ, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	size--
	compressed := br.Next(int(size))
	if len(compressed) != int(size) {
		return nil, errors.New("short block section")
	}
	switch typ {
	case blockTypeUncompressed:
		return bytes.NewBuffer(compressed), nil
	case blockTypeS2:
		dec := s2.NewReader(bytes.NewBuffer(compressed))
		dst.Reset(dec)
		return dst, nil
	case blockTypeZstd:
		err := zDec.Reset(bytes.NewBuffer(compressed))
		if err != nil {
			return nil, err
		}
		dst.Reset(zDec)
		return dst, nil
	}
	return nil, fmt.Errorf("unknown compression type: %d", typ)
}

// indexStrings will deduplicate strings and populate
// strings, stringsMap and stringBuf.
func (s *serializer) indexStrings(sb []byte) error {
	if s.strings == nil {
		s.strings = make(map[string]uint32, 100)
	} else {
		for k := range s.strings {
			delete(s.strings, k)
		}
	}
	// There should be at least 5 bytes between each source,
	// so it should not be possible to alias lookups.
	if cap(s.stringIdxLUT) < len(sb)/4 {
		s.stringIdxLUT = make([]uint32, len(sb)/4)
	}
	s.stringIdxLUT = s.stringIdxLUT[:len(sb)/4]

	if cap(s.stringBuf) == 0 {
		s.stringBuf = make([]byte, 0, len(sb))
	}
	s.stringBuf = s.stringBuf[:0]
	var srcOff, dstOff uint32
	for int(srcOff) < len(sb) {
		length := binary.LittleEndian.Uint32(sb[srcOff : srcOff+4])
		value := sb[srcOff+4 : srcOff+4+length]
		off, ok := s.strings[string(value)]
		if ok {
			s.stringIdxLUT[srcOff/4] = off
			srcOff += 5 + length
			continue
		}
		// New value, add to dst
		s.stringIdxLUT[srcOff/4] = dstOff
		s.stringBuf = append(s.stringBuf, byte(length), byte(length>>8), byte(length>>16), byte(length>>24))
		s.stringBuf = append(s.stringBuf, value...)
		s.stringBuf = append(s.stringBuf, 0)
		s.strings[string(value)] = dstOff
		srcOff += 5 + length
		dstOff += 5 + length
	}
	return nil
}

// indexStrings will deduplicate strings and populate
// strings, stringsMap and stringBuf.
func (s *serializer) indexStringsLazy(sb []byte) error {
	// Only possible on 64 bit platforms, so it will never trigger on 32 bit platforms.
	if uint32(len(sb)) > math.MaxUint32 {
		// This would overflow our offset table
		return nil
	}
	for i := range s.strings2[:] {
		s.strings2[i] = 0
	}
	// There should be at least 5 bytes between each source,
	// so it should not be possible to alias lookups.
	if cap(s.stringIdxLUT) < len(sb)/4 {
		s.stringIdxLUT = make([]uint32, len(sb)/4)
	}
	s.stringIdxLUT = s.stringIdxLUT[:len(sb)/4]
	if cap(s.stringBuf) == 0 {
		s.stringBuf = make([]byte, 0, len(sb))
	}

	s.stringBuf = s.stringBuf[:0]
	var srcOff, dstOff uint32
	for int(srcOff) < len(sb) {
		length := binary.LittleEndian.Uint32(sb[srcOff : srcOff+4])
		value := sb[srcOff+4 : srcOff+4+length]
		h := memHash(value) & stringmask
		off := s.strings2[h]
		if off > 0 {
			off--
			// Does length match?
			if length == binary.LittleEndian.Uint32(s.stringBuf[off:off+4]) {
				// Compare content
				bytes.Equal(value[:], s.stringBuf[off+4:off+4+length])
				s.stringIdxLUT[srcOff/4] = off
				srcOff += 5 + length
				continue
			}
		}
		// New value, add to dst
		s.stringIdxLUT[srcOff/4] = dstOff
		s.stringBuf = append(s.stringBuf, byte(length), byte(length>>8), byte(length>>16), byte(length>>24))
		s.stringBuf = append(s.stringBuf, value...)
		s.stringBuf = append(s.stringBuf, 0)
		s.strings2[h] = dstOff + 1
		srcOff += 5 + length
		dstOff += 5 + length
	}
	return nil
}

const (
	blockTypeUncompressed byte = 0
	blockTypeS2           byte = 1
	blockTypeZstd         byte = 2
)

// compressStringsS2 compresses strings as an s2 block.
func (s *serializer) compressStringsS2() error {
	mel := s2.MaxEncodedLen(len(s.stringBuf)) + 1
	if cap(s.sBuf) < mel {
		s.sBuf = make([]byte, mel)
	}
	s.sBuf = s.sBuf[:mel]
	s.sBuf[0] = blockTypeS2
	sbCopy := s2.Encode(s.sBuf[1:], s.stringBuf)
	s.sBuf = s.sBuf[:len(sbCopy)+1]

	return nil
}

var zDec, _ = zstd.NewReader(nil)
var zEncFast, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderCRC(false))
var zEncNoEnt, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderCRC(false))

func (s *serializer) compressStringsZstd() error {
	mel := len(s.stringBuf) + 1
	if cap(s.sBuf) < mel {
		s.sBuf = make([]byte, mel)
	}
	s.sBuf = s.sBuf[:1]
	s.sBuf[0] = blockTypeZstd
	s.sBuf = zEncFast.EncodeAll(s.stringBuf, s.sBuf)
	//s.sBuf = zEncNoEnt.EncodeAll(s.stringBuf, s.sBuf[:1])

	return nil
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr

// memHash is the hash function used by go map, it utilizes available hardware instructions(behaves
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
