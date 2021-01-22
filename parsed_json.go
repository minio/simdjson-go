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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strconv"
)

//
// For enhanced performance, simdjson-go can point back into the original JSON buffer for strings,
// however this can lead to issues in streaming use cases scenarios, or scenarios in which
// the underlying JSON buffer is reused. So the default behaviour is to create copies of all
// strings (not just those transformed anyway for unicode escape characters) into the separate
// Strings buffer (at the expense of using more memory and less performance).
//
const alwaysCopyStrings = true

const JSONVALUEMASK = 0xffffffffffffff
const JSONTAGMASK = 0xff << 56
const STRINGBUFBIT = 0x80000000000000
const STRINGBUFMASK = 0x7fffffffffffff

const maxdepth = 128

// FloatFlags are flags recorded when converting floats.
type FloatFlags uint64

// FloatFlag is a flag recorded when parsing floats.
type FloatFlag uint64

const (
	// FloatOverflowedInteger is set when number in JSON was in integer notation,
	// but under/overflowed both int64 and uint64 and therefore was parsed as float.
	FloatOverflowedInteger FloatFlag = 1 << iota
)

// Contains returns whether f contains the specified flag.
func (f FloatFlags) Contains(flag FloatFlag) bool {
	return FloatFlag(f)&flag == flag
}

// Flags converts the flag to FloatFlags and optionally merges more flags.
func (f FloatFlag) Flags(more ...FloatFlag) FloatFlags {
	// We operate on a copy, so we can modify f.
	for _, v := range more {
		f |= v
	}
	return FloatFlags(f)
}

type ParsedJson struct {
	Message []byte
	Tape    []uint64
	Strings []byte

	// allows to reuse the internal structures without exposing it.
	internal *internalParsedJson
}

const indexSlots = 16
const indexSize = 1536                            // Seems to be a good size for the index buffering
const indexSizeWithSafetyBuffer = indexSize - 128 // Make sure we never write beyond buffer

type indexChan struct {
	index   int
	length  int
	indexes *[indexSize]uint32
}

type internalParsedJson struct {
	ParsedJson
	containingScopeOffset []uint64
	isvalid               bool
	indexChans            chan indexChan
	indexesChan           indexChan
	buffers               [indexSlots][indexSize]uint32
	buffersOffset         uint64
	ndjson                uint64
}

// Iter returns a new Iter.
func (pj *ParsedJson) Iter() Iter {
	return Iter{tape: *pj}
}

// stringAt returns a string at a specific offset in the stringbuffer.
func (pj *ParsedJson) stringAt(offset, length uint64) (string, error) {
	b, err := pj.stringByteAt(offset, length)
	return string(b), err
}

// stringByteAt returns a string at a specific offset in the stringbuffer.
func (pj *ParsedJson) stringByteAt(offset, length uint64) ([]byte, error) {
	if offset&STRINGBUFBIT == 0 {
		if offset+length > uint64(len(pj.Message)) {
			return nil, fmt.Errorf("string message offset (%v) outside valid area (%v)", offset+length, len(pj.Message))
		}
		return pj.Message[offset : offset+length], nil
	}

	offset = offset & STRINGBUFMASK
	if offset+length > uint64(len(pj.Strings)) {
		return nil, fmt.Errorf("string buffer offset (%v) outside valid area (%v)", offset+length, len(pj.Strings))
	}
	return pj.Strings[offset : offset+length], nil
}

// Iter represents a section of JSON.
// To start iterating it, use Advance() or AdvanceIter() methods
// which will queue the first element.
// If an Iter is copied, the copy will be independent.
type Iter struct {
	// The tape where this iter start.
	tape ParsedJson

	// offset of the next entry to be decoded
	off int

	// addNext is the number of entries to skip for the next entry.
	addNext int

	// current value, exclude tag in top bits
	cur uint64

	// current tag
	t Tag
}

// loadTape will load the input from the supplied readers.
func loadTape(tape, strings io.Reader) (*ParsedJson, error) {
	b, err := ioutil.ReadAll(tape)
	if err != nil {
		return nil, err
	}
	if len(b)&7 != 0 {
		return nil, errors.New("unexpected tape length, should be modulo 8 bytes")
	}
	dst := ParsedJson{
		Tape:    make([]uint64, len(b)/8),
		Strings: nil,
	}
	// Read tape
	for i := range dst.Tape {
		dst.Tape[i] = binary.LittleEndian.Uint64(b[i*8 : i*8+8])
	}
	// Read stringbuf
	b, err = ioutil.ReadAll(strings)
	if err != nil {
		return nil, err
	}
	dst.Strings = b
	return &dst, nil
}

// Advance will read the type of the next element
// and queues up the value on the same level.
func (i *Iter) Advance() Type {
	i.off += i.addNext
	if i.off >= len(i.tape.Tape) {
		i.addNext = 0
		i.t = TagEnd
		return TypeNone
	}

	v := i.tape.Tape[i.off]
	i.cur = v & JSONVALUEMASK
	i.t = Tag(v >> 56)
	i.off++
	i.calcNext(false)
	if i.addNext < 0 {
		// We can't send error, so move to end.
		i.moveToEnd()
		return TypeNone
	}
	return TagToType[i.t]
}

// AdvanceInto will read the tag of the next element
// and move into and out of arrays , objects and root elements.
// This should only be used for strictly manual parsing.
func (i *Iter) AdvanceInto() Tag {
	i.off += i.addNext
	if i.off >= len(i.tape.Tape) {
		i.addNext = 0
		i.t = TagEnd
		return TagEnd
	}

	v := i.tape.Tape[i.off]
	i.cur = v & JSONVALUEMASK
	i.t = Tag(v >> 56)
	i.off++
	i.calcNext(true)
	if i.addNext < 0 {
		// We can't send error, so end tape.
		i.moveToEnd()
		return TagEnd
	}
	return i.t
}

func (i *Iter) moveToEnd() {
	i.off = len(i.tape.Tape)
	i.addNext = 0
	i.t = TagEnd
}

// calcNext will populate addNext to the correct value to skip.
// Specify whether to move into objects/array.
func (i *Iter) calcNext(into bool) {
	i.addNext = 0
	switch i.t {
	case TagInteger, TagUint, TagFloat, TagString:
		i.addNext = 1
	case TagRoot, TagObjectStart, TagArrayStart:
		if !into {
			i.addNext = int(i.cur) - i.off
		}
	}
}

// Type returns the queued value type from the previous call to Advance.
func (i *Iter) Type() Type {
	if i.off+i.addNext > len(i.tape.Tape) {
		return TypeNone
	}
	return TagToType[i.t]
}

// AdvanceIter will read the type of the next element
// and return an iterator only containing the object.
// If dst and i are the same, both will contain the value inside.
func (i *Iter) AdvanceIter(dst *Iter) (Type, error) {
	i.off += i.addNext
	if i.off == len(i.tape.Tape) {
		i.addNext = 0
		i.t = TagEnd
		return TypeNone, nil
	}
	if i.off > len(i.tape.Tape) {
		return TypeNone, errors.New("offset bigger than tape")
	}

	// Get current value off tape.
	v := i.tape.Tape[i.off]
	i.cur = v & JSONVALUEMASK
	i.t = Tag(v >> 56)
	i.off++
	i.calcNext(false)
	if i.addNext < 0 {
		i.moveToEnd()
		return TypeNone, errors.New("element has negative offset")
	}

	// Calculate end of this object.
	iEnd := i.off + i.addNext
	typ := TagToType[i.t]

	// Copy i if different
	if i != dst {
		*dst = *i
	}
	// Move into dst
	dst.calcNext(true)
	if dst.addNext < 0 {
		i.moveToEnd()
		return TypeNone, errors.New("element has negative offset")
	}

	if iEnd > len(dst.tape.Tape) {
		return TypeNone, errors.New("element extends beyond tape")
	}

	// Restrict destination.
	dst.tape.Tape = dst.tape.Tape[:iEnd]

	return typ, nil
}

// PeekNext will return the next value type.
// Returns TypeNone if next ends iterator.
func (i *Iter) PeekNext() Type {
	if i.off+i.addNext >= len(i.tape.Tape) {
		return TypeNone
	}
	return TagToType[Tag(i.tape.Tape[i.off+i.addNext]>>56)]
}

// PeekNextTag will return the tag at the current offset.
// Will return TagEnd if at end of iterator.
func (i *Iter) PeekNextTag() Tag {
	if i.off+i.addNext >= len(i.tape.Tape) {
		return TagEnd
	}
	return Tag(i.tape.Tape[i.off+i.addNext] >> 56)
}

// MarshalJSON will marshal the entire remaining scope of the iterator.
func (i *Iter) MarshalJSON() ([]byte, error) {
	return i.MarshalJSONBuffer(nil)
}

// MarshalJSONBuffer will marshal the remaining scope of the iterator including the current value.
// An optional buffer can be provided for fewer allocations.
// Output will be appended to the destination.
func (i *Iter) MarshalJSONBuffer(dst []byte) ([]byte, error) {
	var tmpBuf []byte

	// Pre-allocate for 100 deep.
	var stackTmp [100]uint8
	// We have a stackNone on top of the stack
	stack := stackTmp[:1]
	const (
		stackNone = iota
		stackArray
		stackObject
		stackRoot
	)

writeloop:
	for {
		// Write key names.
		if stack[len(stack)-1] == stackObject && i.t != TagObjectEnd {
			sb, err := i.StringBytes()
			if err != nil {
				return nil, fmt.Errorf("expected key within object: %w", err)
			}
			dst = append(dst, '"')
			dst = escapeBytes(dst, sb)
			dst = append(dst, '"', ':')
			if i.PeekNextTag() == TagEnd {
				return nil, fmt.Errorf("unexpected end of tape within object")
			}
			i.AdvanceInto()
		}
		//fmt.Println(i.t, len(stack)-1, i.off)
	tagswitch:
		switch i.t {
		case TagRoot:
			isOpenRoot := int(i.cur) > i.off
			if len(stack) > 1 {
				if isOpenRoot {
					return dst, errors.New("root tag open, but not at top of stack")
				}
				l := stack[len(stack)-1]
				switch l {
				case stackRoot:
					if i.PeekNextTag() != TagEnd {
						dst = append(dst, '\n')
					}
					stack = stack[:len(stack)-1]
					break tagswitch
				case stackNone:
					break writeloop
				default:
					return dst, errors.New("root tag, but not at top of stack, got id " + strconv.Itoa(int(l)))
				}
			}

			if isOpenRoot {
				// Always move into root.
				i.addNext = 0
			}
			i.AdvanceInto()
			stack = append(stack, stackRoot)
			continue
		case TagString:
			sb, err := i.StringBytes()
			if err != nil {
				return nil, err
			}
			dst = append(dst, '"')
			dst = escapeBytes(dst, sb)
			dst = append(dst, '"')
			tmpBuf = tmpBuf[:0]
		case TagInteger:
			v, err := i.Int()
			if err != nil {
				return nil, err
			}
			dst = strconv.AppendInt(dst, v, 10)
		case TagUint:
			v, err := i.Uint()
			if err != nil {
				return nil, err
			}
			dst = strconv.AppendUint(dst, v, 10)
		case TagFloat:
			v, err := i.Float()
			if err != nil {
				return nil, err
			}
			dst, err = appendFloat(dst, v)
			if err != nil {
				return nil, err
			}
		case TagNull:
			dst = append(dst, []byte("null")...)
		case TagBoolTrue:
			dst = append(dst, []byte("true")...)
		case TagBoolFalse:
			dst = append(dst, []byte("false")...)
		case TagObjectStart:
			dst = append(dst, '{')
			stack = append(stack, stackObject)
			// We should not emit commas.
			i.AdvanceInto()
			continue
		case TagObjectEnd:
			dst = append(dst, '}')
			if stack[len(stack)-1] != stackObject {
				return dst, errors.New("end of object with no object on stack")
			}
			stack = stack[:len(stack)-1]
		case TagArrayStart:
			dst = append(dst, '[')
			stack = append(stack, stackArray)
			i.AdvanceInto()
			continue
		case TagArrayEnd:
			dst = append(dst, ']')
			if stack[len(stack)-1] != stackArray {
				return nil, errors.New("end of array with no array on stack")
			}
			stack = stack[:len(stack)-1]
		case TagEnd:
			if i.PeekNextTag() == TagEnd {
				return nil, errors.New("no content queued in iterator")
			}
			i.AdvanceInto()
			continue
		}

		if i.PeekNextTag() == TagEnd {
			break
		}
		i.AdvanceInto()

		// Output object separators, etc.
		switch stack[len(stack)-1] {
		case stackArray:
			switch i.t {
			case TagArrayEnd:
			default:
				dst = append(dst, ',')
			}
		case stackObject:
			switch i.t {
			case TagObjectEnd:
			default:
				dst = append(dst, ',')
			}
		}
	}
	if len(stack) > 1 {
		return nil, fmt.Errorf("objects or arrays not closed. left on stack: %v", stack[1:])
	}
	return dst, nil
}

// Float returns the float value of the next element.
// Integers are automatically converted to float.
func (i *Iter) Float() (float64, error) {
	switch i.t {
	case TagFloat:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected float, but no more values on tape")
		}
		v := math.Float64frombits(i.tape.Tape[i.off])
		return v, nil
	case TagInteger:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := int64(i.tape.Tape[i.off])
		return float64(v), nil
	case TagUint:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := i.tape.Tape[i.off]
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unable to convert type %v to float", i.t)
	}
}

// FloatFlags returns the float value of the next element.
// This will include flags from parsing.
// Integers are automatically converted to float.
func (i *Iter) FloatFlags() (float64, FloatFlags, error) {
	switch i.t {
	case TagFloat:
		if i.off >= len(i.tape.Tape) {
			return 0, 0, errors.New("corrupt input: expected float, but no more values on tape")
		}
		v := math.Float64frombits(i.tape.Tape[i.off])
		return v, 0, nil
	case TagInteger:
		if i.off >= len(i.tape.Tape) {
			return 0, 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := int64(i.tape.Tape[i.off])
		return float64(v), 0, nil
	case TagUint:
		if i.off >= len(i.tape.Tape) {
			return 0, 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := i.tape.Tape[i.off]
		return float64(v), FloatFlags(i.cur), nil
	default:
		return 0, 0, fmt.Errorf("unable to convert type %v to float", i.t)
	}
}

// Int returns the integer value of the next element.
// Integers and floats within range are automatically converted.
func (i *Iter) Int() (int64, error) {
	switch i.t {
	case TagFloat:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected float, but no more values on tape")
		}
		v := math.Float64frombits(i.tape.Tape[i.off])
		if v > math.MaxInt64 {
			return 0, errors.New("float value overflows int64")
		}
		if v < math.MinInt64 {
			return 0, errors.New("float value underflows int64")
		}
		return int64(v), nil
	case TagInteger:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := int64(i.tape.Tape[i.off])
		return v, nil
	case TagUint:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := i.tape.Tape[i.off]
		if v > math.MaxInt64 {
			return 0, errors.New("unsigned integer value overflows int64")
		}
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unable to convert type %v to float", i.t)
	}
}

// Uint returns the unsigned integer value of the next element.
// Positive integers and floats within range are automatically converted.
func (i *Iter) Uint() (uint64, error) {
	switch i.t {
	case TagFloat:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected float, but no more values on tape")
		}
		v := math.Float64frombits(i.tape.Tape[i.off])
		if v > math.MaxUint64 {
			return 0, errors.New("float value overflows uint64")
		}
		if v < 0 {
			return 0, errors.New("float value is negative. cannot convert to uint")
		}
		return uint64(v), nil
	case TagInteger:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := int64(i.tape.Tape[i.off])
		if v < 0 {
			return 0, errors.New("integer value is negative. cannot convert to uint")
		}

		return uint64(v), nil
	case TagUint:
		if i.off >= len(i.tape.Tape) {
			return 0, errors.New("corrupt input: expected integer, but no more values on tape")
		}
		v := i.tape.Tape[i.off]
		return v, nil
	default:
		return 0, fmt.Errorf("unable to convert type %v to float", i.t)
	}
}

// String() returns a string value.
func (i *Iter) String() (string, error) {
	if i.t != TagString {
		return "", errors.New("value is not string")
	}
	if i.off >= len(i.tape.Tape) {
		return "", errors.New("corrupt input: no string offset")
	}

	return i.tape.stringAt(i.cur, i.tape.Tape[i.off])
}

// StringBytes() returns a byte array.
func (i *Iter) StringBytes() ([]byte, error) {
	if i.t != TagString {
		return nil, errors.New("value is not string")
	}
	if i.off >= len(i.tape.Tape) {
		return nil, errors.New("corrupt input: no string offset on tape")
	}
	return i.tape.stringByteAt(i.cur, i.tape.Tape[i.off])
}

// StringCvt() returns a string representation of the value.
// Root, Object and Arrays are not supported.
func (i *Iter) StringCvt() (string, error) {
	switch i.t {
	case TagString:
		return i.String()
	case TagInteger:
		v, err := i.Int()
		return strconv.FormatInt(v, 10), err
	case TagUint:
		v, err := i.Uint()
		return strconv.FormatUint(v, 10), err
	case TagFloat:
		v, err := i.Float()
		if err != nil {
			return "", err
		}
		return floatToString(v)
	case TagBoolFalse:
		return "false", nil
	case TagBoolTrue:
		return "true", nil
	case TagNull:
		return "null", nil
	}
	return "", fmt.Errorf("cannot convert type %s to string", TagToType[i.t])
}

// Root() returns the object embedded in root as an iterator
// along with the type of the content of the first element of the iterator.
// An optional destination can be supplied to avoid allocations.
func (i *Iter) Root(dst *Iter) (Type, *Iter, error) {
	if i.t != TagRoot {
		return TypeNone, dst, errors.New("value is not root")
	}
	if i.cur > uint64(len(i.tape.Tape)) {
		return TypeNone, dst, errors.New("root element extends beyond tape")
	}
	if dst == nil {
		c := *i
		dst = &c
	} else {
		dst.cur = i.cur
		dst.off = i.off
		dst.t = i.t
		dst.tape.Strings = i.tape.Strings
		dst.tape.Message = i.tape.Message
	}
	dst.addNext = 0
	dst.tape.Tape = i.tape.Tape[:i.cur-1]
	return dst.AdvanceInto().Type(), dst, nil
}

// Bool() returns the bool value.
func (i *Iter) Bool() (bool, error) {
	switch i.t {
	case TagBoolTrue:
		return true, nil
	case TagBoolFalse:
		return false, nil
	}
	return false, fmt.Errorf("value is not bool, but %v", i.t)
}

// Interface returns the value as an interface.
// Objects are returned as map[string]interface{}.
// Arrays are returned as []interface{}.
// Float values are returned as float64.
// Integer values are returned as int64 or uint64.
// String values are returned as string.
// Boolean values are returned as bool.
// Null values are returned as nil.
// Root objects are returned as []interface{}.
func (i *Iter) Interface() (interface{}, error) {
	switch i.t.Type() {
	case TypeUint:
		return i.Uint()
	case TypeInt:
		return i.Int()
	case TypeFloat:
		return i.Float()
	case TypeNull:
		return nil, nil
	case TypeArray:
		arr, err := i.Array(nil)
		if err != nil {
			return nil, err
		}
		return arr.Interface()
	case TypeString:
		return i.String()
	case TypeObject:
		obj, err := i.Object(nil)
		if err != nil {
			return nil, err
		}
		return obj.Map(nil)
	case TypeBool:
		return i.t == TagBoolTrue, nil
	case TypeRoot:
		var dst []interface{}
		var tmp Iter
		for {
			typ, obj, err := i.Root(&tmp)
			if err != nil {
				return nil, err
			}
			if typ == TypeNone {
				break
			}
			elem, err := obj.Interface()
			if err != nil {
				return nil, err
			}
			dst = append(dst, elem)
			typ = i.Advance()
			if typ != TypeRoot {
				break
			}
		}
		return dst, nil
	case TypeNone:
		if i.PeekNextTag() == TagEnd {
			return nil, errors.New("no content in iterator")
		}
		i.Advance()
		return i.Interface()
	default:
	}
	return nil, fmt.Errorf("unknown tag type: %v", i.t)
}

// Object will return the next element as an object.
// An optional destination can be given.
func (i *Iter) Object(dst *Object) (*Object, error) {
	if i.t != TagObjectStart {
		return nil, errors.New("next item is not object")
	}
	end := i.cur
	if end < uint64(i.off) {
		return nil, errors.New("corrupt input: object ends at index before start")
	}
	if uint64(len(i.tape.Tape)) < end {
		return nil, errors.New("corrupt input: object extended beyond tape")
	}
	if dst == nil {
		dst = &Object{}
	}
	dst.tape.Tape = i.tape.Tape[:end]
	dst.tape.Strings = i.tape.Strings
	dst.tape.Message = i.tape.Message
	dst.off = i.off

	return dst, nil
}

// Array will return the next element as an array.
// An optional destination can be given.
func (i *Iter) Array(dst *Array) (*Array, error) {
	if i.t != TagArrayStart {
		return nil, errors.New("next item is not object")
	}
	end := i.cur
	if uint64(len(i.tape.Tape)) < end {
		return nil, errors.New("corrupt input: object extended beyond tape")
	}
	if dst == nil {
		dst = &Array{}
	}
	dst.tape.Tape = i.tape.Tape[:end]
	dst.tape.Strings = i.tape.Strings
	dst.tape.Message = i.tape.Message
	dst.off = i.off

	return dst, nil
}

func (pj *ParsedJson) Reset() {
	pj.Tape = pj.Tape[:0]
	pj.Strings = pj.Strings[:0]
	pj.Message = pj.Message[:0]
}

func (pj *ParsedJson) get_current_loc() uint64 {
	return uint64(len(pj.Tape))
}

func (pj *ParsedJson) write_tape(val uint64, c byte) {
	pj.Tape = append(pj.Tape, val|(uint64(c)<<56))
}

// writeTapeTagVal will write a tag with no embedded value and a value to the tape.
func (pj *ParsedJson) writeTapeTagVal(tag Tag, val uint64) {
	pj.Tape = append(pj.Tape, uint64(tag)<<56, val)
}

func (pj *ParsedJson) writeTapeTagValFlags(tag Tag, val, flags uint64) {
	pj.Tape = append(pj.Tape, uint64(tag)<<56|flags, val)
}

func (pj *ParsedJson) write_tape_s64(val int64) {
	pj.writeTapeTagVal(TagInteger, uint64(val))
}

func (pj *ParsedJson) write_tape_double(d float64) {
	pj.writeTapeTagVal(TagFloat, math.Float64bits(d))
}

func (pj *ParsedJson) annotate_previousloc(saved_loc uint64, val uint64) {
	pj.Tape[saved_loc] |= val
}

// Tag indicates the data type of a tape entry
type Tag uint8

const (
	TagString      = Tag('"')
	TagInteger     = Tag('l')
	TagUint        = Tag('u')
	TagFloat       = Tag('d')
	TagNull        = Tag('n')
	TagBoolTrue    = Tag('t')
	TagBoolFalse   = Tag('f')
	TagObjectStart = Tag('{')
	TagObjectEnd   = Tag('}')
	TagArrayStart  = Tag('[')
	TagArrayEnd    = Tag(']')
	TagRoot        = Tag('r')
	TagEnd         = Tag(0)
)

var tagOpenToClose = [256]Tag{
	TagObjectStart: TagObjectEnd,
	TagArrayStart:  TagArrayEnd,
	TagRoot:        TagRoot,
}

func (t Tag) String() string {
	return string([]byte{byte(t)})
}

// Type is a JSON value type.
type Type uint8

const (
	TypeNone Type = iota
	TypeNull
	TypeString
	TypeInt
	TypeUint
	TypeFloat
	TypeBool
	TypeObject
	TypeArray
	TypeRoot
)

// String returns the type as a string.
func (t Type) String() string {
	switch t {
	case TypeNone:
		return "(no type)"
	case TypeNull:
		return "null"
	case TypeString:
		return "string"
	case TypeInt:
		return "int"
	case TypeUint:
		return "uint"
	case TypeFloat:
		return "float"
	case TypeBool:
		return "bool"
	case TypeObject:
		return "object"
	case TypeArray:
		return "array"
	case TypeRoot:
		return "root"
	}
	return "(invalid)"
}

// TagToType converts a tag to type.
// For arrays and objects only the start tag will return types.
// All non-existing tags returns TypeNone.
var TagToType = [256]Type{
	TagString:      TypeString,
	TagInteger:     TypeInt,
	TagUint:        TypeUint,
	TagFloat:       TypeFloat,
	TagNull:        TypeNull,
	TagBoolTrue:    TypeBool,
	TagBoolFalse:   TypeBool,
	TagObjectStart: TypeObject,
	TagArrayStart:  TypeArray,
	TagRoot:        TypeRoot,
}

// Type converts a tag to a type.
// Only basic types and array+object start match a type.
func (t Tag) Type() Type {
	return TagToType[t]
}

func (pj *internalParsedJson) dump_raw_tape() bool {

	if !pj.isvalid {
		return false
	}

	for tapeidx := uint64(0); tapeidx < uint64(len(pj.Tape)); tapeidx++ {
		howmany := uint64(0)
		tape_val := pj.Tape[tapeidx]
		ntype := byte(tape_val >> 56)
		fmt.Printf("%d : %c", tapeidx, ntype)

		if ntype == 'r' {
			howmany = tape_val & JSONVALUEMASK
		} else {
			fmt.Errorf("Error: no starting root node?\n")
			return false
		}
		fmt.Printf("\t// pointing to %d (right after last node)\n", howmany)

		// Decrement howmany (since we're adding one now for the ndjson support)
		howmany -= 1

		tapeidx++
		for ; tapeidx < howmany; tapeidx++ {
			tape_val = pj.Tape[tapeidx]
			fmt.Printf("%d : ", tapeidx)
			ntype := Tag(tape_val >> 56)
			payload := tape_val & JSONVALUEMASK
			switch ntype {
			case TagString: // we have a string
				if tapeidx+1 >= howmany {
					return false
				}
				fmt.Printf("string \"")
				tapeidx++
				string_length := pj.Tape[tapeidx]
				str, err := pj.stringAt(payload, string_length)
				if err != nil {
					fmt.Printf("string err:%v\n", err)
					return false
				}
				fmt.Printf("%s (o:%d, l:%d)", print_with_escapes([]byte(str)), payload, string_length)
				fmt.Println("\"")

			case TagInteger: // we have a long int
				if tapeidx+1 >= howmany {
					return false
				}
				tapeidx++
				fmt.Printf("integer %d\n", int64(pj.Tape[tapeidx]))

			case TagFloat: // we have a double
				if tapeidx+1 >= howmany {
					return false
				}
				tapeidx++
				fmt.Printf("float %f\n", math.Float64frombits(pj.Tape[tapeidx]))

			case TagNull: // we have a null
				fmt.Printf("null\n")

			case TagBoolTrue: // we have a true
				fmt.Printf("true\n")

			case TagBoolFalse: // we have a false
				fmt.Printf("false\n")

			case TagObjectStart: // we have an object
				fmt.Printf("{\t// pointing to next Tape location %d (first node after the scope) \n", payload)

			case TagObjectEnd: // we end an object
				fmt.Printf("}\t// pointing to previous Tape location %d (start of the scope) \n", payload)

			case TagArrayStart: // we start an array
				fmt.Printf("\t// pointing to next Tape location %d (first node after the scope) \n", payload)

			case TagArrayEnd: // we end an array
				fmt.Printf("]\t// pointing to previous Tape location %d (start of the scope) \n", payload)

			case TagRoot: // we start and end with the root node
				fmt.Printf("end of root\n")
				return false

			default:
				return false
			}
		}

		tape_val = pj.Tape[tapeidx]
		payload := tape_val & JSONVALUEMASK
		ntype = byte(tape_val >> 56)
		fmt.Printf("%d : %c\t// pointing to %d (start root)\n", tapeidx, ntype, payload)
	}

	return true
}

func print_with_escapes(src []byte) string {
	return string(escapeBytes(make([]byte, 0, len(src)+len(src)>>4), src))
}

// escapeBytes will escape JSON bytes.
// Output is appended to dst.
func escapeBytes(dst, src []byte) []byte {
	for _, s := range src {
		switch s {
		case '\b':
			dst = append(dst, '\\', 'b')

		case '\f':
			dst = append(dst, '\\', 'f')

		case '\n':
			dst = append(dst, '\\', 'n')

		case '\r':
			dst = append(dst, '\\', 'r')

		case '"':
			dst = append(dst, '\\', '"')

		case '\t':
			dst = append(dst, '\\', 't')

		case '\\':
			dst = append(dst, '\\', '\\')

		default:
			if s <= 0x1f {
				dst = append(dst, '\\', 'u', '0', '0', valToHex[s>>4], valToHex[s&0xf])
			} else {
				dst = append(dst, s)
			}
		}
	}

	return dst
}

var valToHex = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

// floatToString converts a float to string similar to Go stdlib.
func floatToString(f float64) (string, error) {
	var tmp [32]byte
	v, err := appendFloat(tmp[:0], f)
	return string(v), err
}

// appendFloat converts a float to string similar to Go stdlib and appends it to dst.
func appendFloat(dst []byte, f float64) ([]byte, error) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, errors.New("INF or NaN number found")
	}

	// Convert as if by ES6 number to string conversion.
	// This matches most other JSON generators.
	// See golang.org/issue/6384 and golang.org/issue/14135.
	// Like fmt %g, but the exponent cutoffs are different
	// and exponents themselves are not padded to two digits.
	abs := math.Abs(f)
	fmt := byte('f')
	if abs != 0 {
		if abs < 1e-6 || abs >= 1e21 {
			fmt = 'e'
		}
	}
	dst = strconv.AppendFloat(dst, f, fmt, -1, 64)
	if fmt == 'e' {
		// clean up e-09 to e-9
		n := len(dst)
		if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst, nil
}
