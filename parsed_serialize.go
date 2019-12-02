package simdjson

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
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

type Serializer struct {
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

	compValues, compTags uint8
	alwaysZstdStrings    bool
	reIndexStrings       bool
}

func NewSerializer() *Serializer {
	s := Serializer{
		compValues:     blockTypeS2,
		compTags:       blockTypeS2,
		reIndexStrings: true,
	}
	return &s
}

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
	var reIndexed bool
	if s.reIndexStrings {
		reIndexed = s.indexStringsLazy(pj.Strings)
		//fmt.Println("strings dedupe:", len(pj.Strings), "->", len(s.stringBuf))
	} else {
		s.stringBuf = pj.Strings
	}
	var wg sync.WaitGroup
	wg.Add(1)

	// Choose zstd when tape is likely to take longer than strings.
	zstdStrings := len(s.stringBuf) < len(pj.Tape)*10
	go func() {
		defer wg.Done()
		if zstdStrings || s.alwaysZstdStrings {
			s.sBuf = encBlock(blockTypeZstd, s.stringBuf, s.sBuf)
		} else {
			s.sBuf = encBlock(blockTypeS2, s.stringBuf, s.sBuf)
		}
	}()

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
			if reIndexed {
				binary.LittleEndian.PutUint64(tmp[:], uint64(s.stringIdxLUT[uint32(payload)/4]))
			} else {
				binary.LittleEndian.PutUint64(tmp[:], payload)
			}
			s.valuesBuf = append(s.valuesBuf, tmp[:]...)
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
		default:
			wg.Wait()
			panic(fmt.Errorf("unknown tag: %v", ntype))
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
	// Tape elements, uncompressed.
	n = binary.PutUvarint(tmp[:], uint64(len(pj.Tape)))
	dst = append(dst, tmp[:n]...)

	// Size of varints...
	varInts := binary.PutUvarint(tmp[:], uint64(len(s.sBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.tagsBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.tagsCompBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.valuesBuf))) +
		binary.PutUvarint(tmp[:], uint64(len(s.valuesCompBuf)))

	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)+len(s.tagsCompBuf)+len(s.valuesCompBuf)+varInts))
	dst = append(dst, tmp[:n]...)

	// Strings
	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.sBuf...)

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
		fmt.Println("strings:", len(pj.Strings), "->", len(s.sBuf), "tags:", len(s.tagsBuf), "->", len(s.tagsCompBuf), "values:", len(s.valuesBuf), "->", len(s.valuesCompBuf), "Total:", len(pj.Strings)+len(pj.Tape)*8, "->", len(dst))
	}

	return dst
}

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
	values := s.valuesBuf
	for _, t := range s.tagsBuf {
		if off == len(dst.Tape) {
			return dst, errors.New("tags extended beyond tape")
		}
		tag := Tag(t)

		tagDst := uint64(t) << 56
		switch tag {
		case TagString:
			if len(values) < 8 {
				return dst, fmt.Errorf("reading %v: no values left", tag)
			}
			sOffset := binary.LittleEndian.Uint64(values[:8])
			values = values[8:]
			if sOffset > uint64(len(dst.Strings)-5) {
				return dst, fmt.Errorf("%v extends beyond stringbuf (%d). offset:%d", tag, len(dst.Strings), sOffset)
			}

			dst.Tape[off] = tagDst | sOffset
			off++
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

// indexStrings will deduplicate strings and populate
// strings, stringsMap and stringBuf.
func (s *Serializer) indexStrings(sb []byte) error {
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
func (s *Serializer) indexStringsLazy(sb []byte) bool {
	// Only possible on 64 bit platforms, so it will never trigger on 32 bit platforms.
	if uint32(len(sb)) > math.MaxUint32 {
		s.stringBuf = sb
		// This would overflow our offset table.
		return false
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
	return true
}

const (
	blockTypeUncompressed byte = 0
	blockTypeS2           byte = 1
	blockTypeZstd         byte = 2
)

var zDec, _ = zstd.NewReader(nil)
var zEncFast, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderCRC(false))

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
		mel := s2.MaxEncodedLen(len(src)) + 1
		if cap(dst) < mel {
			dst = make([]byte, mel)
		}
		dst = dst[:mel]
		dst[0] = mode
		got := s2.Encode(dst[1:], src)
		return dst[:len(got)+1]
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
