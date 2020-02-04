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
func __find_whitespace_and_structurals()

//go:noescape
func _find_whitespace_and_structurals(input, whitespace, structurals unsafe.Pointer)

func find_whitespace_and_structurals(buf []byte, whitespace, structurals *uint64) {
	_find_whitespace_and_structurals(unsafe.Pointer(&buf[0]), unsafe.Pointer(whitespace), unsafe.Pointer(structurals))
}
