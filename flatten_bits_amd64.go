//+build !noasm
//+build !appengine

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
	"unsafe"
)

//go:noescape
func __flatten_bits_incremental()

//go:noescape
func _flatten_bits_incremental(base_ptr, pbase unsafe.Pointer, mask uint64, carried unsafe.Pointer, position unsafe.Pointer)

func flatten_bits_incremental(base *[INDEX_SIZE]uint32, base_index *int, mask uint64, carried *int, position *uint64) {
	_flatten_bits_incremental(unsafe.Pointer(&(*base)[0]), unsafe.Pointer(base_index), mask, unsafe.Pointer(carried), unsafe.Pointer(position))
}
