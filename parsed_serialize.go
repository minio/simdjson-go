package simdjson

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	stringsMap map[uint32]uint32
	stringBuf  []byte

	// Compressed strings
	sBuf []byte
	// Uncompressed tags
	tagsBuf []byte
	// Values
	valuesBuf     []byte
	valuesCompBuf []byte
	strings2      [stringSize]uint32
}

func (s *serializer) Serialize(dst []byte, pj ParsedJson) ([]byte, error) {
	// Header: Version byte
	// Strings:
	// - Varint: Compressed bytes total.
	//  Compressed Blocks:
	//     - Varint: Block compressed bytes
	//     - Block type:
	//     		0: uncompressed, rest is data.
	//     		1: RLE, data: 1 value, varuint: length
	//     		2: huff0 with table.
	// 			3: S2 block.
	// 	   - block data.
	// 	   Ends when total is reached.
	// Varuint: Tape length, Unique Elements
	// Tags: FSE compressed block
	// - Byte type:
	// 		- 0: Uncompressed.
	// 		- 1: RLE: 1 value, varuint: length
	// 		- 2: FSE compressed with table.
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
	err := s.compressStringsS2()
	if err != nil {
		return nil, err
	}
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
		off++
		switch ntype {
		case TagString:
			var sOffset uint32
			if reIndexStrings {
				var ok bool
				sOffset, ok = s.stringsMap[uint32(payload)]
				if !ok {
					return nil, fmt.Errorf("unable to find string at offset %d", payload)
				}
			} else {
				sOffset = uint32(payload)
			}
			s.tagsBuf[tagsOff] = symbolString
			n := binary.PutUvarint(tmp[:], uint64(sOffset))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagUint:
			s.tagsBuf[tagsOff] = symbolUint
			n := binary.PutUvarint(tmp[:], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
			off++
		case TagInteger:
			s.tagsBuf[tagsOff] = symbolInteger
			n := binary.PutVarint(tmp[:], int64(pj.Tape[off]))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
			off++
		case TagFloat:
			s.tagsBuf[tagsOff] = symbolFloat
			binary.LittleEndian.PutUint64(tmp[:8], pj.Tape[off])
			s.valuesBuf = append(s.valuesBuf, tmp[:8]...)
			off++
		case TagNull:
			s.tagsBuf[tagsOff] = symbolNull
		case TagBoolTrue:
			s.tagsBuf[tagsOff] = symbolBoolTrue
		case TagBoolFalse:
			s.tagsBuf[tagsOff] = symbolBoolFalse
		case TagObjectStart:
			s.tagsBuf[tagsOff] = symbolObjectStart
			n := binary.PutUvarint(tmp[:], payload-uint64(off))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagObjectEnd:
			s.tagsBuf[tagsOff] = symbolObjectEnd
			n := binary.PutUvarint(tmp[:], uint64(off)-payload)
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagArrayStart:
			s.tagsBuf[tagsOff] = symbolArrayStart
			n := binary.PutUvarint(tmp[:], payload-uint64(off))
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagArrayEnd:
			s.tagsBuf[tagsOff] = symbolArrayEnd
			n := binary.PutUvarint(tmp[:], uint64(off)-payload)
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)
		case TagRoot:
			s.tagsBuf[tagsOff] = symbolRoot
			n := binary.PutUvarint(tmp[:], payload)
			s.valuesBuf = append(s.valuesBuf, tmp[:n]...)

		default:
			return nil, fmt.Errorf("unknown tag: %v", ntype)
		}
		tagsOff++
	}

	s.tagsBuf[tagsOff] = symbolEnd
	s.tagsBuf = s.tagsBuf[:tagsOff]
	s.tComp.MaxSymbolValue = symbolEnd + 1
	s.tComp.TableLog = 7
	var compHeader [binary.MaxVarintLen64 + 2]byte
	var compHeaderLen int
	comp, err := fse.Compress(s.tagsBuf, &s.tComp)
	switch err {
	case fse.ErrUseRLE:
		comp = nil
		compHeader[0] = 1
		compHeader[1] = s.tagsBuf[0]
		compHeaderLen = 2 + binary.PutUvarint(compHeader[2:], uint64(len(s.tagsBuf)))
	case fse.ErrIncompressible:
		comp = s.tagsBuf
		compHeader[0] = 0
		compHeaderLen = 1 + binary.PutUvarint(compHeader[1:], uint64(len(s.tagsBuf)))
	case nil:
		compHeader[0] = 2
		compHeaderLen = 1 + binary.PutUvarint(compHeader[1:], uint64(len(comp)))
	default:
		return nil, err
	}

	// S2 compress values
	mel := s2.MaxEncodedLen(len(s.valuesBuf))
	if cap(s.valuesCompBuf) < mel {
		s.valuesCompBuf = make([]byte, mel)
	}
	s.valuesCompBuf = s.valuesCompBuf[:mel]
	s.valuesCompBuf = s2.Encode(s.valuesCompBuf, s.valuesBuf)

	// Version
	dst = append(dst, 1)
	// Size
	// TODO: Doesn't include the varints
	n := binary.PutUvarint(tmp[:], uint64(len(s.sBuf)+len(comp)+len(s.valuesBuf)+compHeaderLen))
	dst = append(dst, tmp[:n]...)

	// Strings
	n = binary.PutUvarint(tmp[:], uint64(len(s.sBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.sBuf...)

	// Tags
	dst = append(dst, compHeader[:compHeaderLen]...)
	if len(comp) > 0 {
		dst = append(dst, comp...)
	}

	// Values
	n = binary.PutUvarint(tmp[:], uint64(len(s.valuesCompBuf)))
	dst = append(dst, tmp[:n]...)
	dst = append(dst, s.valuesCompBuf...)
	if false {
		fmt.Println("strings:", len(pj.Strings), "->", len(s.sBuf), "tags:", len(s.tagsBuf), "->", len(comp), "values:", len(s.valuesBuf), "->", len(s.valuesCompBuf), "Total:", len(pj.Strings)+len(pj.Tape)*8, "->", len(dst))
	}

	return dst, nil
}

const (
	symbolString = iota
	symbolInteger
	symbolUint
	symbolFloat
	symbolNull
	symbolBoolTrue
	symbolBoolFalse
	symbolObjectStart
	symbolObjectEnd
	symbolArrayStart
	symbolArrayEnd
	symbolRoot
	symbolEnd
)

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
	if s.stringsMap == nil {
		s.stringsMap = make(map[uint32]uint32, 100)
	} else {
		for k := range s.stringsMap {
			delete(s.stringsMap, k)
		}
	}
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
			s.stringsMap[srcOff] = off
			srcOff += 5 + length
			continue
		}
		// New value, add to dst
		s.stringsMap[srcOff] = dstOff
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
	for i := range s.strings2[:] {
		s.strings2[i] = 0
	}
	if s.stringsMap == nil {
		s.stringsMap = make(map[uint32]uint32, 1000)
	} else {
		for k := range s.stringsMap {
			delete(s.stringsMap, k)
		}
	}
	if cap(s.stringBuf) == 0 {
		s.stringBuf = make([]byte, 0, len(sb))
	}
	s.stringBuf = s.stringBuf[:0]
	var srcOff, dstOff uint32
	//var key [32]byte
	for int(srcOff) < len(sb) {
		length := binary.LittleEndian.Uint32(sb[srcOff : srcOff+4])
		value := sb[srcOff+4 : srcOff+4+length]
		//h := highwayhash.Sum64(value, key[:]) & stringmask
		h := memHash(value) & stringmask
		off := s.strings2[h]
		if off > 0 {
			off--
			// Does length match?
			if length == binary.LittleEndian.Uint32(s.stringBuf[off:off+4]) {
				bytes.Equal(value[:], s.stringBuf[off+4:off+4+length])
				s.stringsMap[srcOff] = off
				srcOff += 5 + length
				continue
			}
		}
		// New value, add to dst
		s.stringsMap[srcOff] = dstOff
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
	blockTypeUncompressed = 0
	blockTypeRLE          = 1
	blockTypeHuff0Table   = 2
	blockTypeS2           = 3
)

// compressStrings huff0 compresses strings.
// worse than s2, should probably be removed.
func (s *serializer) compressStrings() error {
	if cap(s.sBuf) < len(s.stringBuf) {
		s.sBuf = make([]byte, len(s.stringBuf))
	}

	const stringBlockSize = 64 << 10
	compress := s.stringBuf
	s.sBuf = s.sBuf[:0]
	s.sComp.Reuse = huff0.ReusePolicyNone
	var tmp [binary.MaxVarintLen64]byte
	var tmp2 [binary.MaxVarintLen64]byte
	for len(compress) > 0 {
		todo := compress
		if len(todo) > stringBlockSize {
			todo = todo[:stringBlockSize]
		}
		compress = compress[len(todo):]
		out, _, err := huff0.Compress1X(todo, &s.sComp)
		switch err {
		case nil:
			n := binary.PutUvarint(tmp[:], uint64(len(out)+1))
			s.sBuf = append(s.sBuf, tmp[:n]...)
			s.sBuf = append(s.sBuf, blockTypeHuff0Table)
			s.sBuf = append(s.sBuf, out...)
		case huff0.ErrUseRLE:
			n := binary.PutUvarint(tmp[:], uint64(len(todo)))
			n2 := binary.PutUvarint(tmp2[:], uint64(n+2))
			s.sBuf = append(s.sBuf, tmp2[:n2]...)
			s.sBuf = append(s.sBuf, blockTypeRLE, todo[0])
			s.sBuf = append(s.sBuf, tmp[:n]...)
		case huff0.ErrIncompressible:
			n := binary.PutUvarint(tmp[:], uint64(len(todo)+1))
			s.sBuf = append(s.sBuf, tmp[:n]...)
			s.sBuf = append(s.sBuf, blockTypeUncompressed)
			s.sBuf = append(s.sBuf, todo...)
		default:
			return err
		}
	}
	return nil
}

// compressStringsS2 compresses strings as an s2 block.
func (s *serializer) compressStringsS2() error {
	mel := s2.MaxEncodedLen(len(s.stringBuf)) + 1
	if cap(s.sBuf) < mel {
		s.sBuf = make([]byte, mel)
	}
	s.sBuf = s.sBuf[:mel]
	s.sBuf[0] = blockTypeS2
	_ = s2.Encode(s.sBuf[1:], s.stringBuf)

	return nil
}

var zEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))

func (s *serializer) compressStringsZstd() error {
	mel := len(s.stringBuf)
	if cap(s.sBuf) < mel {
		s.sBuf = make([]byte, mel)
	}
	s.sBuf = zEnc.EncodeAll(s.stringBuf, s.sBuf[:0])

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
