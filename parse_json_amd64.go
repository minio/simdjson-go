//+build !noasm
//+build !appengine
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
	"bytes"
	"errors"
	"sync"
)

func (pj *internalParsedJson) initialize(size int) {
	// Estimate the tape size to be about 15% of the length of the JSON message
	avgTapeSize := size * 15 / 100
	if cap(pj.Tape) < avgTapeSize {
		pj.Tape = make([]uint64, 0, avgTapeSize)
	}
	pj.Tape = pj.Tape[:0]

	stringsSize := size / 10
	if stringsSize < 128 {
		stringsSize = 128 // always allocate at least 128 for the string buffer
	}
	if pj.Strings != nil && cap(pj.Strings.B) >= stringsSize {
		pj.Strings.B = pj.Strings.B[:0]
	} else {
		pj.Strings = &TStrings{make([]byte, 0, stringsSize)}
	}
	if cap(pj.containingScopeOffset) < maxdepth {
		pj.containingScopeOffset = make([]uint64, 0, maxdepth)
	}
	pj.containingScopeOffset = pj.containingScopeOffset[:0]
}

func (pj *internalParsedJson) parseMessage(msg []byte, ndjson bool) (err error) {
	// Cache message so we can point directly to strings
	// TODO: Find out why TestVerifyTape/instruments fails without bytes.TrimSpace
	pj.Message = bytes.TrimSpace(msg)
	pj.initialize(len(pj.Message))

	if ndjson {
		pj.ndjson = 1
	} else {
		pj.ndjson = 0
	}

	// Make the capacity of the channel smaller than the number of slots.
	// This way the sender will automatically block until the consumer
	// has finished the slot it is working on.
	pj.indexChans = make(chan indexChan, indexSlots-2)
	pj.buffersOffset = ^uint64(0)

	var errStage1 error

	// Do long inputs async
	if len(pj.Message) > 8<<10 {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !pj.unifiedMachine(pj.Message) {
				err = errors.New("Bad parsing while executing stage 2")
				// drain the channel until empty
				for range pj.indexChans {
				}
			}
		}()
		if !pj.findStructuralIndices(pj.Message) {
			errStage1 = errors.New("Failed to find all structural indices for stage 1")
		}
		wg.Wait()
	} else {
		if !pj.findStructuralIndices(pj.Message) {
			// drain the channel until empty
			for range pj.indexChans {
			}
			return errors.New("Failed to find all structural indices for stage 1")
		}
		if !pj.unifiedMachine(pj.Message) {
			// drain the channel until empty
			for range pj.indexChans {
			}
			return errors.New("Bad parsing while executing stage 2")
		}
		return nil
	}

	if errStage1 != nil {
		return errStage1
	}
	return
}
