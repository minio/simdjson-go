// +build !amd64 appengine !gc noasm

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
	"io"
)

// SupportedCPU will return whether the CPU is supported.
func SupportedCPU() bool {
	return false
}

// Parse a block of data and return the parsed JSON.
// An optional block of previously parsed json can be supplied to reduce allocations.
func Parse(b []byte, reuse *ParsedJson) (*ParsedJson, error) {
	return nil, errors.New("Unsupported platform")
}

// ParseND will parse newline delimited JSON.
// An optional block of previously parsed json can be supplied to reduce allocations.
func ParseND(b []byte, reuse *ParsedJson) (*ParsedJson, error) {
	return nil, errors.New("Unsupported platform")
}

// A Stream is used to stream back results.
// Either Error or Value will be set on returned results.
type Stream struct {
	Value *ParsedJson
	Error error
}

// ParseNDStream will parse a stream and return parsed JSON to the supplied result channel.
// The method will return immediately.
// Each element is contained within a root tag.
//   <root>Element 1</root><root>Element 2</root>...
// Each result will contain an unspecified number of full elements,
// so it can be assumed that each result starts and ends with a root tag.
// The parser will keep parsing until writes to the result stream blocks.
// A stream is finished when a non-nil Error is returned.
// If the stream was parsed until the end the Error value will be io.EOF
// The channel will be closed after an error has been returned.
// An optional channel for returning consumed results can be provided.
// There is no guarantee that elements will be consumed, so always use
// non-blocking writes to the reuse channel.
func ParseNDStream(r io.Reader, res chan<- Stream, reuse <-chan *ParsedJson) {
	go func() {
		res <- Stream{
			Value: nil,
			Error: fmt.Errorf("Unsupported platform"),
		}
		close(res)
	}()
	return
}
