//+build !appengine
//+build !noasm
//+build gc

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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/cpuid/v2"
)

// SupportedCPU will return whether the CPU is supported.
func SupportedCPU() bool {
	return cpuid.CPU.Supports(cpuid.AVX2, cpuid.CLMUL)
}

// Parse a block of data and return the parsed JSON.
// An optional block of previously parsed json can be supplied to reduce allocations.
func Parse(b []byte, reuse *ParsedJson) (*ParsedJson, error) {
	if !SupportedCPU() {
		return nil, errors.New("Host CPU does not meet target specs")
	}
	var pj *internalParsedJson
	if reuse != nil && reuse.internal != nil {
		pj = reuse.internal
		pj.ParsedJson = *reuse
		pj.ParsedJson.internal = nil
		reuse = &ParsedJson{}
	}
	if pj == nil {
		pj = &internalParsedJson{}
	}
	err := pj.parseMessage(b)
	if err != nil {
		return nil, err
	}
	parsed := &pj.ParsedJson
	parsed.internal = pj
	return parsed, nil
}

// ParseND will parse newline delimited JSON.
// An optional block of previously parsed json can be supplied to reduce allocations.
func ParseND(b []byte, reuse *ParsedJson) (*ParsedJson, error) {
	if !SupportedCPU() {
		return nil, errors.New("Host CPU does not meet target specs")
	}
	var pj internalParsedJson
	if reuse != nil {
		pj.ParsedJson = *reuse
	}
	b = bytes.TrimSpace(b)

	err := pj.parseMessageNdjson(b)
	if err != nil {
		return nil, err
	}
	return &pj.ParsedJson, nil
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
	if !SupportedCPU() {
		go func() {
			res <- Stream{
				Value: nil,
				Error: fmt.Errorf("Host CPU does not meet target specs"),
			}
			close(res)
		}()
		return
	}
	const tmpSize = 10 << 20
	buf := bufio.NewReaderSize(r, tmpSize)
	tmpPool := sync.Pool{New: func() interface{} {
		return make([]byte, tmpSize+1024)
	}}
	conc := (runtime.GOMAXPROCS(0) + 1) / 2
	queue := make(chan chan Stream, conc)
	go func() {
		// Forward finished items in order.
		defer close(res)
		end := false
		for items := range queue {
			i := <-items
			select {
			case res <- i:
			default:
				if !end {
					// Block if we haven't returned an error
					res <- i
				}
			}
			if i.Error != nil {
				end = true
			}
		}
	}()
	go func() {
		defer close(queue)
		for {
			tmp := tmpPool.Get().([]byte)
			tmp = tmp[:tmpSize]
			n, err := buf.Read(tmp)
			if err != nil && err != io.EOF {
				queueError(queue, err)
				return
			}
			tmp = tmp[:n]
			// Read until Newline
			if err != io.EOF {
				b, err2 := buf.ReadBytes('\n')
				if err2 != nil && err2 != io.EOF {
					queueError(queue, err2)
					return
				}
				tmp = append(tmp, b...)
				// Forward io.EOF
				err = err2
			}

			if len(tmp) > 0 {
				result := make(chan Stream, 0)
				queue <- result
				go func() {
					var pj internalParsedJson
					select {
					case v := <-reuse:
						if cap(v.Message) >= tmpSize+1024 {
							tmpPool.Put(v.Message)
							v.Message = nil
						}
						pj.ParsedJson = *v

					default:
					}
					parseErr := pj.parseMessageNdjson(tmp)
					if parseErr != nil {
						result <- Stream{
							Value: nil,
							Error: fmt.Errorf("parsing input: %w", parseErr),
						}
						return
					}
					parsed := pj.ParsedJson
					result <- Stream{
						Value: &parsed,
						Error: nil,
					}
				}()
			} else {
				tmpPool.Put(tmp)
			}
			if err != nil {
				// Should only really be io.EOF
				queueError(queue, err)
				return
			}
		}
	}()
}

func queueError(queue chan chan Stream, err error) {
	result := make(chan Stream, 0)
	queue <- result
	result <- Stream{
		Value: nil,
		Error: err,
	}
}
