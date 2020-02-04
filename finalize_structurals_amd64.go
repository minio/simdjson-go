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
func __finalize_structurals()

//go:noescape
func _finalize_structurals(structurals_in, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred unsafe.Pointer) (structurals uint64)

func finalize_structurals(structurals, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred *uint64) uint64 {
	return _finalize_structurals(structurals, whitespace, quote_mask, quote_bits, unsafe.Pointer(prev_iter_ends_pseudo_pred))
}
