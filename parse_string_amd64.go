//go:build !noasm && !appengine && gc
// +build !noasm,!appengine,gc

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
	"reflect"
	"unsafe"
)

//go:noescape
func _parse_string_validate_only(src, maxStringSize, str_length, dst_length unsafe.Pointer) (result uint64)

//go:noescape
func _parse_string(src, dst, pcurrent_string_buf_loc unsafe.Pointer) (res uint64)

func parseStringSimdValidateOnly(buf []byte, maxStringSize, dstLength *uint64, needCopy *bool) bool {

	src := unsafe.Pointer(&buf[1]) // Use buf[1] in order to skip opening quote
	src_length := uint64(0)

	success := _parse_string_validate_only(src, unsafe.Pointer(&maxStringSize), unsafe.Pointer(&src_length), unsafe.Pointer(dstLength))

	*needCopy = *needCopy || src_length != *dstLength
	return success != 0
}

func parseStringSimd(buf []byte, stringbuf *[]byte) bool {

	sh := (*reflect.SliceHeader)(unsafe.Pointer(stringbuf))
	sb := *stringbuf
	sb = append(sb, 0)

	src := unsafe.Pointer(&buf[1]) // Use buf[1] in order to skip opening quote
	string_buf_loc := unsafe.Pointer(uintptr(unsafe.Pointer(&sb[0])) + uintptr(sh.Len)*unsafe.Sizeof(sb[0]))
	dst := string_buf_loc

	res := _parse_string(src, dst, unsafe.Pointer(&string_buf_loc))

	sh.Len += int(uintptr(string_buf_loc) - uintptr(dst))

	return res != 0
}
