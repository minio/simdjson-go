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
)

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
			return dst, err
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

// FindKey will return a single named element.
// An optional destination can be given.
// The method will return nil if the element cannot be found.
// This should only be used to locate a single key where the object is no longer needed.
// The object will not be advanced.
func (o *Object) FindKey(key string, dst *Element) *Element {
	tmp := o.tape.Iter()
	tmp.off = o.off
	for {
		typ := tmp.Advance()
		// We want name and at least one value.
		if typ != TypeString || tmp.off+1 >= len(tmp.tape.Tape) {
			return nil
		}
		// Advance must be string or end of object
		offset := tmp.cur
		length := tmp.tape.Tape[tmp.off]
		if int(length) != len(key) {
			// Skip the value.
			t := tmp.Advance()
			if t == TypeNone {
				return nil
			}
			continue
		}
		// Read name
		name, err := tmp.tape.stringByteAt(offset, length)
		if err != nil {
			return nil
		}

		if string(name) != key {
			// Skip the value
			tmp.Advance()
			continue
		}
		if dst == nil {
			dst = &Element{}
		}
		dst.Name = key
		dst.Type, err = tmp.AdvanceIter(&dst.Iter)
		if err != nil {
			return nil
		}
		return dst
	}
}

// NextElement sets dst to the next element and returns the name.
// TypeNone with nil error will be returned if there are no more elements.
func (o *Object) NextElement(dst *Iter) (name string, t Type, err error) {
	n, t, err := o.NextElementBytes(dst)
	return string(n), t, err
}

// NextElementBytes sets dst to the next element and returns the name.
// TypeNone with nil error will be returned if there are no more elements.
// Contrary to NextElement this will not cause allocations.
func (o *Object) NextElementBytes(dst *Iter) (name []byte, t Type, err error) {
	if o.off >= len(o.tape.Tape) {
		return nil, TypeNone, nil
	}
	// Advance must be string or end of object
	v := o.tape.Tape[o.off]
	switch Tag(v >> 56) {
	case TagString:
		// Read name:
		// We want name and at least one value.
		if o.off+2 >= len(o.tape.Tape) {
			return nil, TypeNone, fmt.Errorf("parsing object element name: unexpected end of tape")
		}
		length := o.tape.Tape[o.off+1]
		offset := v & JSONVALUEMASK
		name, err = o.tape.stringByteAt(offset, length)
		if err != nil {
			return nil, TypeNone, fmt.Errorf("parsing object element name: %w", err)
		}
		o.off += 2
	case TagObjectEnd:
		return nil, TypeNone, nil
	default:
		return nil, TypeNone, fmt.Errorf("object: unexpected tag %c", byte(v>>56))
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
		return nil, TypeNone, errors.New("element extends beyond tape")
	}
	dst.tape.Tape = dst.tape.Tape[:dst.off+elemSize]

	// Skip to next element
	o.off += elemSize
	return name, TagToType[dst.t], nil
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
