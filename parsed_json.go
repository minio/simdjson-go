package simdjson

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"sync"
	"strconv"
)

//
// The current parse_number() code has some precision issues
// (see PR "Exact float parsing", https://github.com/lemire/simdjson/pull/333)
//
// An example of this (from canada.json):
//     "-65.619720000000029" --> -65.61972000000004
// instead of
//     "-65.619720000000029" --> -65.61972000000003
//
// There is a slow code path that uses Golang's ParseFloat (disabled by default)
//
const SLOWGOLANGFLOATPARSING = false

const JSONVALUEMASK = 0xffffffffffffff
const DEFAULTMAXDEPTH = 128

type ParsedJson struct {
	Tape    []uint64
	Strings []byte
}

const INDEX_SIZE = 4096 // Seems to be a good size for the index buffering

type indexChan struct {
	length  int
	indexes *[INDEX_SIZE]uint32
}

type internalParsedJson struct {
	ParsedJson
	containing_scope_offset []uint64
	isvalid                 bool
	index_chan				chan indexChan
}

func (pj *internalParsedJson) initialize(size int) {
	pj.Tape = make([]uint64, 0, size)
	// FIXME(fwessel): Why are string size the same as element size?
	pj.Strings = make([]byte, 0, size)
	pj.containing_scope_offset = make([]uint64, 0, DEFAULTMAXDEPTH)
}

func (pj *internalParsedJson) parseMessage(msg []byte) error {

	//TODO: Collect errors from both stages and return appropriately
	var wg sync.WaitGroup
	wg.Add(2)

	pj.index_chan = make(chan indexChan, 16)

	go func() {
		find_structural_indices(msg, pj)
		wg.Done()
	}()
	go func() {
		unified_machine(msg, pj)
		wg.Done()
	}()

	wg.Wait()

	return nil
}

// Iter returns a new Iter
func (pj *ParsedJson) Iter() Iter {
	return Iter{tape: *pj}
}

// stringAt returns a string at a specific offset in the stringbuffer.
func (pj *ParsedJson) stringAt(offset uint64) (string, error) {
	b, err := pj.stringByteAt(offset)
	return string(b), err
}

// stringByteAt returns a string at a specific offset in the stringbuffer.
func (pj *ParsedJson) stringByteAt(offset uint64) ([]byte, error) {
	offset &= JSONVALUEMASK
	// There must be at least 4 byte length and one 0 byte.
	if offset+5 > uint64(len(pj.Strings)) {
		return nil, errors.New("string offset outside valid area")
	}
	length := uint64(binary.LittleEndian.Uint32(pj.Strings[offset : offset+4]))
	if offset+length+4 > uint64(len(pj.Strings)) {
		return nil, errors.New("string offset+length outside valid area")
	}
	return pj.Strings[offset+4 : offset+4+length], nil
}

// Iter represents a section of JSON.
// To start iterating it, use Next() or NextIter() methods
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

// LoadTape will load the input from the supplied readers.
func LoadTape(tape, strings io.Reader) (*ParsedJson, error) {
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

// Next will read the type of the next element
// and queues up the value on the same level.
func (i *Iter) Next() Type {
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

// NextInto will read the tag of the next element
// and move into and out of arrays , objects and root elements.
// This should only be used for strictly manual parsing.
func (i *Iter) NextInto() Tag {
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
	case TagInteger, TagUint, TagFloat:
		i.addNext = 1
	case TagRoot, TagObjectStart, TagArrayStart:
		if !into {
			i.addNext = int(i.cur) - i.off
		}
	}
}

// Type returns the queued value type from the previous call to Next.
func (i *Iter) Type() Type {
	if i.off+i.addNext > len(i.tape.Tape) {
		return TypeNone
	}
	return TagToType[i.t]
}

// NextIter will read the type of the next element
// and return an iterator only containing the object.
// If dst and i are the same, both will contain the value inside.
func (i *Iter) NextIter(dst *Iter) (Type, error) {
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

// MarshalJSONBuffer will marshal the entire remaining scope of the iterator.
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
	)

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
			i.NextInto()
		}

		switch i.t {
		case TagRoot:
			// Move into root.
			var err error
			i, err = i.Root()
			if err != nil {
				return nil, err
			}
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
			i.NextInto()
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
			i.NextInto()
			continue
		case TagArrayEnd:
			dst = append(dst, ']')
			if stack[len(stack)-1] != stackArray {
				return nil, errors.New("end of object with no array on stack")
			}
			stack = stack[:len(stack)-1]
		}

		if i.PeekNextTag() == TagEnd {
			break
		}
		i.NextInto()

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
	return i.tape.stringAt(i.cur)
}

// StringBytes() returns a byte array.
func (i *Iter) StringBytes() ([]byte, error) {
	if i.t != TagString {
		return nil, errors.New("value is not string")
	}
	return i.tape.stringByteAt(i.cur)
}

// StringCvt() returns a string representation of the value.
// Root, Object and Arrays are not supported.
func (i *Iter) StringCvt() (string, error) {
	switch i.t {
	case TagString:
		return i.tape.stringAt(i.cur)
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

// Root() returns the object embedded in root as an iterator.
func (i *Iter) Root() (*Iter, error) {
	if i.t != TagRoot {
		return nil, errors.New("value is not string")
	}
	if i.cur > uint64(len(i.tape.Tape)) {
		return nil, errors.New("root element extends beyond tape")
	}
	dst := Iter{}
	dst = *i
	dst.addNext = 0
	dst.tape.Tape = dst.tape.Tape[:i.cur]
	return &dst, nil
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
		// Skip root
		obj, err := i.Root()
		if err != nil {
			return nil, err
		}
		return obj.Interface()
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
	dst.off = i.off

	return dst, nil
}

// Object represents a JSON object.
type Object struct {
	// Complete tape
	tape ParsedJson

	// offset of the next entry to be decoded
	off int
}

// Map will unmarshal into a map[string]interface{}
// See Iter.Interface() for a reference on value types.
func (o *Object) Map(dst map[string]interface{}) (map[string]interface{}, error) {
	if dst == nil {
		dst = make(map[string]interface{})
	}
	var tmp Iter
	for {
		name, t, err := o.NextElement(&tmp)
		if err != nil {
			return nil, err
		}
		if t == TypeNone {
			// Done
			break
		}
		dst[name], err = tmp.Interface()
		if err != nil {
			return nil, fmt.Errorf("parsing element %q: %w", name, err)
		}
	}
	return dst, nil
}

// Element represents an element in an object.
type Element struct {
	// Name of the element
	Name string
	// Type of the element
	Type Type
	// Iter containing the element
	Iter Iter
}

// Elements contains all elements in an object
// kept in original order.
// And index contains lookup for object keys.
type Elements struct {
	Elements []Element
	Index    map[string]int
}

// Lookup a key in elements and return the element.
// Returns nil if key doesn't exist.
// Keys are case sensitive.
func (e Elements) Lookup(key string) *Element {
	idx, ok := e.Index[key]
	if !ok {
		return nil
	}
	return &e.Elements[idx]
}

// MarshalJSON will marshal the entire remaining scope of the iterator.
func (e Elements) MarshalJSON() ([]byte, error) {
	return e.MarshalJSONBuffer(nil)
}

// MarshalJSONBuffer will marshal all elements.
// An optional buffer can be provided for fewer allocations.
// Output will be appended to the destination.
func (e Elements) MarshalJSONBuffer(dst []byte) ([]byte, error) {
	dst = append(dst, '{')
	for i, elem := range e.Elements {
		dst = append(dst, '"')
		dst = escapeBytes(dst, []byte(elem.Name))
		dst = append(dst, '"', ':')
		var err error
		dst, err = elem.Iter.MarshalJSONBuffer(dst)
		if err != nil {
			return nil, err
		}
		if i < len(e.Elements)-1 {
			dst = append(dst, ',')
		}
	}
	dst = append(dst, '}')
	return dst, nil
}

// Parse will return all elements and iterators.
// An optional destination can be given.
// The Object will be consumed.
func (o *Object) Parse(dst *Elements) (*Elements, error) {
	if dst == nil {
		dst = &Elements{
			Elements: make([]Element, 0, 5),
			Index:    make(map[string]int, 5),
		}
	} else {
		dst.Elements = dst.Elements[:0]
		for k := range dst.Index {
			delete(dst.Index, k)
		}
	}
	var tmp Iter
	for {
		name, t, err := o.NextElement(&tmp)
		if err != nil {
			return nil, err
		}
		if t == TypeNone {
			// Done
			break
		}
		dst.Index[name] = len(dst.Elements)
		dst.Elements = append(dst.Elements, Element{
			Name: name,
			Type: t,
			Iter: tmp,
		})
	}
	return dst, nil
}

// NextElement sets dst to the next element and returns the name.
// TypeNone with nil error will be returned if there are no more elements.
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error) {
	if o.off >= len(o.tape.Tape) {
		return "", TypeNone, nil
	}
	// Next must be string or end of object
	v := o.tape.Tape[o.off]
	switch Tag(v >> 56) {
	case TagString:
		// Read name:
		name, err = o.tape.stringAt(v)
		if err != nil {
			return "", TypeNone, fmt.Errorf("parsing object element name: %w", err)
		}
		o.off++
	case TagObjectEnd:
		return "", TypeNone, nil
	default:
		return "", TypeNone, fmt.Errorf("object: unexpected tag %v", string(v>>56))
	}

	// Read element type
	v = o.tape.Tape[o.off]
	// Move to value (if any)
	o.off++

	// Set dst
	dst.cur = v & JSONVALUEMASK
	dst.t = Tag(v >> 56)
	dst.off = o.off
	dst.tape = o.tape
	dst.calcNext(false)
	elemSize := dst.addNext
	dst.calcNext(true)
	if dst.off+elemSize > len(dst.tape.Tape) {
		return "", TypeNone, errors.New("element extends beyond tape")
	}
	dst.tape.Tape = dst.tape.Tape[:dst.off+elemSize]

	// Skip to next element
	o.off += elemSize
	return name, TagToType[dst.t], nil
}

// Array represents a JSON array.
// There are methods that allows to get full arrays if the value type is the same.
// Otherwise an iterator can be retrieved.
type Array struct {
	tape ParsedJson
	off  int
}

// Iter returns the array as an iterator.
// This can be used for parsing mixed content arrays.
// The first value is ready with a call to Next.
// Calling after last element should have TypeNone.
func (a *Array) Iter() Iter {
	i := Iter{
		tape: a.tape,
		off:  a.off,
	}
	return i
}

// FirstType will return the type of the first element.
// If there are no elements, TypeNone is returned.
func (a *Array) FirstType() Type {
	iter := a.Iter()
	return iter.PeekNext()
}

// MarshalJSON will marshal the entire remaining scope of the iterator.
func (a *Array) MarshalJSON() ([]byte, error) {
	return a.MarshalJSONBuffer(nil)
}

// MarshalJSONBuffer will marshal all elements.
// An optional buffer can be provided for fewer allocations.
// Output will be appended to the destination.
func (a *Array) MarshalJSONBuffer(dst []byte) ([]byte, error) {
	dst = append(dst, '[')
	i := a.Iter()
	var elem Iter
	for {
		t, err := i.NextIter(&elem)
		if err != nil {
			return nil, err
		}
		if t == TypeNone {
			break
		}
		dst, err = elem.MarshalJSONBuffer(dst)
		if err != nil {
			return nil, err
		}
		if i.PeekNextTag() == TagArrayEnd {
			break
		}
		dst = append(dst, ',')
	}
	if i.PeekNextTag() != TagArrayEnd {
		return nil, errors.New("expected TagArrayEnd as final tag in array")
	}
	dst = append(dst, ']')
	return dst, nil
}

// Interface returns the array as a slice of interfaces.
// See Iter.Interface() for a reference on value types.
func (a *Array) Interface() ([]interface{}, error) {
	// Estimate length. Assume one value per element.
	lenEst := (len(a.tape.Tape) - a.off - 1) / 2
	if lenEst < 0 {
		lenEst = 0
	}
	dst := make([]interface{}, 0, lenEst)
	i := a.Iter()
	for i.Next() != TypeNone {
		elem, err := i.Interface()
		if err != nil {
			return nil, err
		}
		dst = append(dst, elem)
	}
	return dst, nil
}

// AsFloat returns the array values as float.
// Integers are automatically converted to float.
func (a *Array) AsFloat() ([]float64, error) {
	// Estimate length
	lenEst := (len(a.tape.Tape) - a.off - 1) / 2
	if lenEst < 0 {
		lenEst = 0
	}
	dst := make([]float64, 0, lenEst)

readArray:
	for {
		tag := Tag(a.tape.Tape[a.off] >> 56)
		a.off++
		switch tag {
		case TagFloat:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected float, but no more values")
			}
			dst = append(dst, math.Float64frombits(a.tape.Tape[a.off]))
		case TagInteger:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected integer, but no more values")
			}
			dst = append(dst, float64(int64(a.tape.Tape[a.off])))
		case TagUint:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected integer, but no more values")
			}
			dst = append(dst, float64(a.tape.Tape[a.off]))
		case TagArrayEnd:
			break readArray
		default:
			return nil, fmt.Errorf("unable to convert type %v to float", tag)
		}
		a.off++
	}
	return dst, nil
}

// AsInteger returns the array values as float.
// Integers are automatically converted to float.
func (a *Array) AsInteger() ([]int64, error) {
	// Estimate length
	lenEst := (len(a.tape.Tape) - a.off - 1) / 2
	if lenEst < 0 {
		lenEst = 0
	}
	dst := make([]int64, 0, lenEst)
readArray:
	for {
		tag := Tag(a.tape.Tape[a.off] >> 56)
		a.off++
		switch tag {
		case TagFloat:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected float, but no more values")
			}
			val := math.Float64frombits(a.tape.Tape[a.off])
			if val > math.MaxInt64 {
				return nil, errors.New("float value overflows int64")
			}
			if val < math.MinInt64 {
				return nil, errors.New("float value underflows int64")
			}
			dst = append(dst, int64(val))
		case TagInteger:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected integer, but no more values")
			}
			dst = append(dst, int64(a.tape.Tape[a.off]))
		case TagUint:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected integer, but no more values")
			}

			val := a.tape.Tape[a.off]
			if val > math.MaxInt64 {
				return nil, errors.New("unsigned integer value overflows int64")
			}

			dst = append(dst)
		case TagArrayEnd:
			break readArray
		default:
			return nil, fmt.Errorf("unable to convert type %v to integer", tag)
		}
		a.off++
	}
	return dst, nil
}

// AsString returns the array values as a slice of strings.
// No conversion is done.
func (a *Array) AsString() ([]string, error) {
	// Estimate length
	lenEst := len(a.tape.Tape) - a.off - 1
	if lenEst < 0 {
		lenEst = 0
	}
	dst := make([]string, 0, lenEst)
	i := a.Iter()
	var elem Iter
	for {
		t, err := i.NextIter(&elem)
		if err != nil {
			return nil, err
		}
		switch t {
		case TypeNone:
			return dst, nil
		case TypeString:
			s, err := elem.String()
			if err != nil {

			}
			dst = append(dst, s)
		default:
			return nil, fmt.Errorf("element in array is not string, but %v", t)
		}
	}
}

func (pj *ParsedJson) Reset() {
	pj.Tape = pj.Tape[:0]
	pj.Strings = pj.Strings[:0]
}

func (pj *ParsedJson) get_current_loc() uint64 {
	return uint64(len(pj.Tape))
}

func (pj *ParsedJson) write_tape(val uint64, c byte) {
	pj.Tape = append(pj.Tape, val|(uint64(c)<<56))
}

func (pj *ParsedJson) write_tape_s64(val int64) {
	pj.write_tape(0, 'l')
	pj.Tape = append(pj.Tape, uint64(val))
}

func (pj *ParsedJson) write_tape_double(d float64) {
	pj.write_tape(0, 'd')
	pj.Tape = append(pj.Tape, math.Float64bits(d))
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

func (pj *ParsedJson) dump_raw_tape() bool {

	//if !pj.isvalid {
	//	return false
	// }

	tapeidx := uint64(0)
	howmany := uint64(0)
	tape_val := pj.Tape[tapeidx]
	ntype := tape_val >> 56
	fmt.Printf("%d : %s", tapeidx, string(ntype))

	if ntype == 'r' {
		howmany = tape_val & JSONVALUEMASK
	} else {
		fmt.Errorf("Error: no starting root node?\n")
		return false
	}
	fmt.Printf("\t// pointing to %d (right after last node)\n", howmany)

	tapeidx++
	for ; tapeidx < howmany; tapeidx++ {
		tape_val = pj.Tape[tapeidx]
		fmt.Printf("%d : ", tapeidx)
		ntype := Tag(tape_val >> 56)
		payload := tape_val & JSONVALUEMASK
		switch ntype {
		case TagString: // we have a string
			fmt.Printf("string \"")
			string_length := uint64(binary.LittleEndian.Uint32(pj.Strings[payload : payload+4]))
			fmt.Printf("%s", print_with_escapes(pj.Strings[payload+4:payload+4+string_length]))
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
	ntype = tape_val >> 56
	fmt.Printf("%d : %s\t// pointing to %d (start root)\n", tapeidx, string(ntype), payload)

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
