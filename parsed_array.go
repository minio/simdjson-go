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
)

// Array represents a JSON array.
// There are methods that allows to get full arrays if the value type is the same.
// Otherwise an iterator can be retrieved.
type Array struct {
	tape ParsedJson
	off  int
}

// Iter returns the array as an iterator.
// This can be used for parsing mixed content arrays.
// The first value is ready with a call to Advance.
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
		t, err := i.AdvanceIter(&elem)
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
	for i.Advance() != TypeNone {
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

// AsInteger returns the array values as int64 values.
// Uints/Floats are automatically converted to int64 if they fit within the range.
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

// AsUint64 returns the array values as float.
// Uints/Floats are automatically converted to uint64 if they fit within the range.
func (a *Array) AsUint64() ([]uint64, error) {
	// Estimate length
	lenEst := (len(a.tape.Tape) - a.off - 1) / 2
	if lenEst < 0 {
		lenEst = 0
	}
	dst := make([]uint64, 0, lenEst)
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
				return nil, errors.New("float value overflows uint64")
			}
			if val < 0 {
				return nil, errors.New("float value is negative")
			}
			dst = append(dst, uint64(val))
		case TagInteger:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected integer, but no more values")
			}
			val := int64(a.tape.Tape[a.off])
			if val < 0 {
				return nil, errors.New("int64 value is negative")
			}
			dst = append(dst, uint64(val))
		case TagUint:
			if len(a.tape.Tape) <= a.off {
				return nil, errors.New("corrupt input: expected integer, but no more values")
			}

			dst = append(dst, a.tape.Tape[a.off])
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
		t, err := i.AdvanceIter(&elem)
		if err != nil {
			return nil, err
		}
		switch t {
		case TypeNone:
			return dst, nil
		case TypeString:
			s, err := elem.String()
			if err != nil {
				return nil, err
			}
			dst = append(dst, s)
		default:
			return nil, fmt.Errorf("element in array is not string, but %v", t)
		}
	}
}

// AsStringCvt returns the array values as a slice of strings.
// Scalar types are converted.
// Root, Object and Arrays are not supported an will return an error if found.
func (a *Array) AsStringCvt() ([]string, error) {
	// Estimate length
	lenEst := len(a.tape.Tape) - a.off - 1
	if lenEst < 0 {
		lenEst = 0
	}
	dst := make([]string, 0, lenEst)
	i := a.Iter()
	var elem Iter
	for {
		t, err := i.AdvanceIter(&elem)
		if err != nil {
			return nil, err
		}
		switch t {
		case TypeNone:
			return dst, nil
		default:
			s, err := elem.StringCvt()
			if err != nil {
				return nil, err
			}
			dst = append(dst, s)
		}
	}
}
