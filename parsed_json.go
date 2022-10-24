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
	"errors"
	"fmt"
	"math"
	"strconv"
)

const JSONVALUEMASK = 0xff_ffff_ffff_ffff
const JSONTAGOFFSET = 56
const JSONTAGMASK = 0xff << JSONTAGOFFSET
const STRINGBUFBIT = 0x80_0000_0000_0000
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

type TStrings struct {
	B []byte
}

type ParsedJson struct {
	Message []byte
	Tape    []uint64
	Strings *TStrings

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
	copyStrings           bool
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
	if offset+length > uint64(len(pj.Strings.B)) {
		return nil, fmt.Errorf("string buffer offset (%v) outside valid area (%v)", offset+length, len(pj.Strings.B))
	}
	return pj.Strings.B[offset : offset+length], nil
}

// ForEach returns each line in NDJSON, or the top element in non-ndjson.
// This will usually be an object or an array.
// If the callback returns a non-nil error parsing stops and the errors is returned.
func (pj *ParsedJson) ForEach(fn func(i Iter) error) error {
	i := Iter{tape: *pj}
	var elem Iter
	for {
		t, err := i.AdvanceIter(&elem)
		if err != nil || t != TypeRoot {
			return err
		}
		elem.AdvanceInto()
		if err = fn(elem); err != nil {
			return err
		}
	}
}

// Clone returns a deep clone of the ParsedJson.
// If a nil destination is sent a new will be created.
func (pj *ParsedJson) Clone(dst *ParsedJson) *ParsedJson {
	if dst == nil {
		dst = &ParsedJson{
			Message:  make([]byte, len(pj.Message)),
			Tape:     make([]uint64, len(pj.Tape)),
			Strings:  &TStrings{make([]byte, len(pj.Strings.B))},
			internal: nil,
		}
	} else {
		if cap(dst.Message) < len(pj.Message) {
			dst.Message = make([]byte, len(pj.Message))
		}
		if cap(dst.Tape) < len(pj.Tape) {
			dst.Tape = make([]uint64, len(pj.Tape))
		}
		if dst.Strings == nil {
			dst.Strings = &TStrings{make([]byte, len(pj.Strings.B))}
		} else if cap(dst.Strings.B) < len(pj.Strings.B) {
			dst.Strings.B = make([]byte, len(pj.Strings.B))
		}
	}
	dst.internal = nil
	dst.Tape = dst.Tape[:len(pj.Tape)]
	copy(dst.Tape, pj.Tape)
	dst.Message = dst.Message[:len(pj.Message)]
	copy(dst.Message, pj.Message)
	dst.Strings.B = dst.Strings.B[:len(pj.Strings.B)]
	copy(dst.Strings.B, pj.Strings.B)
	return dst
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

// Advance will read the type of the next element
// and queues up the value on the same level.
func (i *Iter) Advance() Type {
	i.off += i.addNext

	for {
		if i.off >= len(i.tape.Tape) {
			i.addNext = 0
			i.t = TagEnd
			return TypeNone
		}

		v := i.tape.Tape[i.off]
		i.t = Tag(v >> 56)
		i.off++
		i.cur = v & JSONVALUEMASK
		if i.t == TagNop {
			i.off += int(i.cur)
			continue
		}
		break
	}
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
	for {
		if i.off >= len(i.tape.Tape) {
			i.addNext = 0
			i.t = TagEnd
			return TagEnd
		}

		v := i.tape.Tape[i.off]
		i.t = Tag(v >> 56)
		i.cur = v & JSONVALUEMASK
		if i.t == TagNop {
			if i.cur <= 0 {
				i.moveToEnd()
				return TagEnd
			}
			i.off += int(i.cur)
			continue
		}
		i.off++
		break
	}
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

	// Get current value off tape.
	for {
		if i.off == len(i.tape.Tape) {
			i.addNext = 0
			i.t = TagEnd
			return TypeNone, nil
		}
		if i.off > len(i.tape.Tape) {
			return TypeNone, errors.New("offset bigger than tape")
		}

		v := i.tape.Tape[i.off]
		i.cur = v & JSONVALUEMASK
		i.t = Tag(v >> 56)
		i.off++
		if i.t == TagNop {
			if i.cur <= 0 {
				return TypeNone, errors.New("invalid nop skip")
			}
			i.off += int(i.cur)
			continue
		}
		break
	}
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
	off := i.off + i.addNext
	for {
		if off >= len(i.tape.Tape) {
			return TypeNone
		}
		v := i.tape.Tape[off]
		t := Tag(v >> 56)
		if t == TagNop {
			skip := int(v & JSONVALUEMASK)
			if skip <= 0 {
				return TypeNone
			}
			off += skip
			continue
		}
		return TagToType[t]
	}
}

// PeekNextTag will return the tag at the current offset.
// Will return TagEnd if at end of iterator.
func (i *Iter) PeekNextTag() Tag {
	off := i.off + i.addNext
	for {
		if off >= len(i.tape.Tape) {
			return TagEnd
		}
		v := i.tape.Tape[off]
		t := Tag(v >> 56)
		if t == TagNop {
			skip := int(v & JSONVALUEMASK)
			if skip <= 0 {
				return TagEnd
			}
			off += skip
			continue
		}
		return t
	}
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
		// Copy so "stack" doesn't escape.
		sCopy := append(make([]uint8, 0, len(stack)-1), stack[1:]...)
		return nil, fmt.Errorf("objects or arrays not closed. left on stack: %v", sCopy)
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
		return v, FloatFlags(i.cur), nil
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
		return float64(v), 0, nil
	default:
		return 0, 0, fmt.Errorf("unable to convert type %v to float", i.t)
	}
}

// SetFloat can change a float, int, uint or string with the specified value.
// Attempting to change other types will return an error.
func (i *Iter) SetFloat(v float64) error {
	switch i.t {
	case TagFloat, TagInteger, TagUint, TagString:
		i.tape.Tape[i.off-1] = uint64(TagFloat) << JSONTAGOFFSET
		i.tape.Tape[i.off] = math.Float64bits(v)
		i.t = TagFloat
		i.cur = 0
		return nil
	}
	return fmt.Errorf("cannot set tag %s to float", i.t.String())
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
		return 0, fmt.Errorf("unable to convert type %v to int", i.t)
	}
}

// SetInt can change a float, int, uint or string with the specified value.
// Attempting to change other types will return an error.
func (i *Iter) SetInt(v int64) error {
	switch i.t {
	case TagFloat, TagInteger, TagUint, TagString:
		i.tape.Tape[i.off-1] = uint64(TagInteger) << JSONTAGOFFSET
		i.tape.Tape[i.off] = uint64(v)
		i.t = TagInteger
		i.cur = uint64(v)
		return nil
	}
	return fmt.Errorf("cannot set tag %s to int", i.t.String())
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
		return 0, fmt.Errorf("unable to convert type %v to uint", i.t)
	}
}

// SetUInt can change a float, int, uint or string with the specified value.
// Attempting to change other types will return an error.
func (i *Iter) SetUInt(v uint64) error {
	switch i.t {
	case TagString, TagFloat, TagInteger, TagUint:
		i.tape.Tape[i.off-1] = uint64(TagUint) << JSONTAGOFFSET
		i.tape.Tape[i.off] = v
		i.t = TagUint
		i.cur = v
		return nil
	}
	return fmt.Errorf("cannot set tag %s to uint", i.t.String())
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

// StringBytes returns a string as byte array.
func (i *Iter) StringBytes() ([]byte, error) {
	if i.t != TagString {
		return nil, errors.New("value is not string")
	}
	if i.off >= len(i.tape.Tape) {
		return nil, errors.New("corrupt input: no string offset on tape")
	}
	return i.tape.stringByteAt(i.cur, i.tape.Tape[i.off])
}

// SetString can change a string, int, uint or float with the specified string.
// Attempting to change other types will return an error.
func (i *Iter) SetString(v string) error {
	return i.SetStringBytes([]byte(v))
}

// SetStringBytes can change a string, int, uint or float with the specified string.
// Attempting to change other types will return an error.
// Sending nil will add an empty string.
func (i *Iter) SetStringBytes(v []byte) error {
	switch i.t {
	case TagString, TagFloat, TagInteger, TagUint:
		i.cur = ((uint64(TagString) << JSONTAGOFFSET) | STRINGBUFBIT) | uint64(len(i.tape.Strings.B))
		i.tape.Tape[i.off-1] = i.cur
		i.tape.Tape[i.off] = uint64(len(v))
		i.t = TagString
		i.tape.Strings.B = append(i.tape.Strings.B, v...)
		return nil
	}
	return fmt.Errorf("cannot set tag %s to string", i.t.String())
}

// StringCvt returns a string representation of the value.
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

// Root returns the object embedded in root as an iterator
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

// FindElement allows searching for fields and objects by path from the iter and forward,
// moving into root and objects, but not arrays.
// For example "Image", "Url" will search the current root/object for an "Image"
// object and return the value of the "Url" element.
// ErrPathNotFound is returned if any part of the path cannot be found.
// If the tape contains an error it will be returned.
// The iter will *not* be advanced.
func (i *Iter) FindElement(dst *Element, path ...string) (*Element, error) {
	if len(path) == 0 {
		return dst, ErrPathNotFound
	}
	// Local copy.
	cp := *i
	for {
		switch cp.t {
		case TagObjectStart:
			var o Object
			obj, err := cp.Object(&o)
			if err != nil {
				return dst, err
			}
			return obj.FindPath(dst, path...)
		case TagRoot:
			_, _, err := cp.Root(&cp)
			if err != nil {
				return dst, err
			}
			continue
		case TagEnd:
			tag := cp.AdvanceInto()
			if tag == TagEnd {
				return dst, ErrPathNotFound
			}
			continue
		default:
			return dst, fmt.Errorf("type %q found before object was found", cp.t)
		}
	}
}

// Bool returns the bool value.
func (i *Iter) Bool() (bool, error) {
	switch i.t {
	case TagBoolTrue:
		return true, nil
	case TagBoolFalse:
		return false, nil
	}
	return false, fmt.Errorf("value is not bool, but %v", i.t)
}

// SetBool can change a bool or null type to bool with the specified value.
// Attempting to change other types will return an error.
func (i *Iter) SetBool(v bool) error {
	switch i.t {
	case TagBoolTrue, TagBoolFalse, TagNull:
		if v {
			i.t = TagBoolTrue
			i.cur = 0
			i.tape.Tape[i.off-1] = uint64(TagBoolTrue) << JSONTAGOFFSET
		} else {
			i.t = TagBoolFalse
			i.cur = 0
			i.tape.Tape[i.off-1] = uint64(TagBoolFalse) << JSONTAGOFFSET
		}
		return nil
	}
	return fmt.Errorf("cannot set tag %s to bool", i.t.String())
}

// SetNull can change the following types to null:
// Bool, String, (Unsigned) Integer, Float, Objects and Arrays.
// Attempting to change other types will return an error.
func (i *Iter) SetNull() error {
	switch i.t {
	case TagBoolTrue, TagBoolFalse, TagNull:
		// 1 value on stream
		i.t = TagNull
		i.cur = 0
		i.tape.Tape[i.off-1] = uint64(TagNull) << JSONTAGOFFSET
	case TagString, TagFloat, TagInteger, TagUint:
		// 2 values
		i.tape.Tape[i.off-1] = uint64(TagNull) << JSONTAGOFFSET
		i.tape.Tape[i.off] = uint64(TagNop)<<JSONTAGOFFSET | 1
		i.t = TagNull
		i.cur = 0
	case TagObjectStart, TagArrayStart, TagRoot:
		// Read length, skipping the object/array:
		i.addNext = int(i.cur) - i.off
		i.tape.Tape[i.off-1] = uint64(TagNull) << JSONTAGOFFSET
		// Fill with nops
		for j := i.off; j < int(i.cur); j++ {
			i.tape.Tape[j] = uint64(TagNop)<<JSONTAGOFFSET | (i.cur - uint64(j))
		}
		i.t = TagNull
		i.cur = 0
	default:
		return fmt.Errorf("cannot set tag %s to null", i.t.String())
	}
	return nil
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
	pj.Strings.B = pj.Strings.B[:0]
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

func (pj *ParsedJson) writeTapeTagValFlags(id, val uint64) {
	pj.Tape = append(pj.Tape, id, val)
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
	TagNop         = Tag('N')
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

var shouldEscape = [256]bool{
	'\b': true,
	'\f': true,
	'\n': true,
	'\r': true,
	'"':  true,
	'\t': true,
	'\\': true,
	// Remaining will be added in init below.
}

func init() {
	for i := range shouldEscape[:0x20] {
		shouldEscape[i] = true
	}
}

// escapeBytes will escape JSON bytes.
// Output is appended to dst.
func escapeBytes(dst, src []byte) []byte {
	esc := false
	for i, s := range src {
		if shouldEscape[s] {
			if i > 0 {
				dst = append(dst, src[:i]...)
				src = src[i:]
			}
			esc = true
			break
		}
	}
	if !esc {
		// Nothing was escaped...
		return append(dst, src...)
	}
	for _, s := range src {
		if !shouldEscape[s] {
			dst = append(dst, s)
			continue
		}
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
			dst = append(dst, '\\', 'u', '0', '0', valToHex[s>>4], valToHex[s&0xf])
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
	if (abs >= 1e-6 && abs < 1e21) || abs == 0 {
		return appendFloatF(dst, f), nil
	}
	dst = strconv.AppendFloat(dst, f, 'e', -1, 64)
	// clean up e-09 to e-9
	n := len(dst)
	if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
		dst[n-2] = dst[n-1]
		dst = dst[:n-1]
	}
	return dst, nil
}
