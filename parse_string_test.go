package simdjson

import (
	"bytes"
	"fmt"
	"testing"
)

func TestParseString(t *testing.T) {

	tests := []struct {
		name    string
		str     string
		success bool
		want    []byte
	}{
		{
			name:    "simple1",
			str:     `a`,
			success: true,
			want:    []byte(`a`),
		},
		{
			name:    "unicode-euro",
			str:     `\u20AC`,
			success: true,
			want:    []byte("€"),
		},
		{
			name:    "unicode-too-short",
			str:     `\u20A`,
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// enclose test string in quotes (as validated by stage 1)
			buf := []byte(fmt.Sprintf(`"%s"`, tt.str))
			dest := make([]byte, 0, 5+len(buf))

			success := parse_string_simd(buf, &dest)

			if success != tt.success {
				t.Errorf("TestParseString() got = %v, want %v", success, tt.success)
			}
			if success {
				size := len(dest) - 4 - 1
				if size != len(tt.want) {
					t.Errorf("TestParseString() got = %d, want %d", size, len(tt.want))
				}
				if bytes.Compare(dest[4:4+size], tt.want) != 0 {
					t.Errorf("TestParseString() got = %v, want %v", string(dest[4:4+size]), tt.want)
				}
			}
		})
	}
}

func TestParseStringValidateOnly(t *testing.T) {

	tests := []struct {
		name    string
		str     string
		success bool
		want    []byte
	}{
		{
			name:    "ascii-1",
			str:     `a`,
			success: true,
			want:    []byte(`a`),
		},
		{
			name:    "ascii-2",
			str:     `ba`,
			success: true,
			want:    []byte(`ba`),
		},
		{
			name:    "ascii-3",
			str:     `cba`,
			success: true,
			want:    []byte(`cba`),
		},
		{
			name:    "ascii-long",
			str:     `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`,
			success: true,
			want:    []byte(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`),
		},
		{
			name:    "unicode-1",
			str:     `\u1234`,
			success: true,
			want:    []byte{225, 136, 180},
		},
		{
			name:    "unicode-short-by-1",
			str:     `\u123`,
			success: false,
		},
		{
			name:    "unicode-short-by-2",
			str:     `\u12`,
			success: false,
		},
		{
			name:    "unicode-short-by-3",
			str:     `\u1`,
			success: false,
		},
		{
			name:    "unicode-short-by-4",
			str:     `\u`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane",
			str:     `\udbff\u1234`,
			success: true,
			want:    []byte{239, 184, 180},
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-1",
			str:     `\udbff\u123`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-2",
			str:     `\udbff\u12`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-3",
			str:     `\udbff\u1`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-4",
			str:     `\udbff\u`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-5",
			str:     `\udbff\`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-6",
			str:     `\udbff`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-7",
			str:     `\udbf`,
			success: false,
		},
		{
			name:    "outside-basic-multilingual-plane-short-by-8",
			str:     `\udbf`,
			success: false,
		},
		{
			name:    "quote1",
			str:     `a\"b`,
			success: true,
			want:    []byte{97, 34, 98},
		},
		{
			name:    "quote2",
			str:     `a\"b\"c`,
			success: true,
			want:    []byte{97, 34, 98, 34, 99},
		},
		{
			name: "unicode-1-seq",
			str: `\u0123`,
			success: true,
			want: []byte{196, 163},
		},
		{
			name: "unicode-2-seqs",
			str: `\u0123\u4567`,
			success: true,
			want: []byte{196, 163, 228, 149, 167},
		},
		{
			name: "unicode-3-seqs",
			str: `\u0123\u4567\u89AB`,
			success: true,
			want: []byte{196, 163, 228, 149, 167, 232, 166, 171},
		},
		{
			name: "unicode-4-seqs",
			str: `\u0123\u4567\u89AB\uCDEF`,
			success: true,
			want: []byte{196, 163, 228, 149, 167, 232, 166, 171, 236, 183, 175},
		},
		{
			name:    "uni1-end-of-ymm-word",
			str:     `---------9---------9\udbff\u1234`,
			success: true,
			want:    []byte(string(`---------9---------9`) + string([]byte{0xef, 0xb8, 0xb4})),
		},
		{
			name:    "uni1-end-of-ymm-word-pass-one-beyond",
			str:     `---------9---------9-\udbff\u1234`,
			success: true,
			want:    []byte(string(`---------9---------9-`) + string([]byte{0xef, 0xb8, 0xb4})),
		},
		{
			name:    "uni1-end-of-ymm-word-pass-two-beyond",
			str:     `---------9---------9--\udbff\u1234`,
			success: true,
			want:    []byte(string(`---------9---------9--`) + string([]byte{0xef, 0xb8, 0xb4})),
		},
		{
			name:    "uni1-end-of-ymm-word-pass-three-beyond",
			str:     `---------9---------9---\udbff\u1234`,
			success: true,
			want:    []byte(string(`---------9---------9---`) + string([]byte{0xef, 0xb8, 0xb4})),
		},
		{
			name:    "uni1-end-of-ymm-word-fail-one-beyond",
			str:     `---------9---------9-\udbff\u123`,
			success: false,
		},
		{
			name:    "uni1-end-of-ymm-word-pass-two-beyond",
			str:     `---------9---------9--\udbff\u123`,
			success: false,
		},
		{
			name:    "uni1-end-of-ymm-word-fail-three-beyond",
			str:     `---------9---------9---\udbff\u123`,
			success: false,
		},
		{
			name:    "uni1-end-of-ymm-word-single",
			str:     `---------9---------9------\u20ac`,
			success: true,
			want:    []byte(string(`---------9---------9------`) + string([]byte{0xe2, 0x82, 0xac})),
		},
		{
			name:    "uni1-end-of-ymm-word-single-pass-one-beyond",
			str:     `---------9---------9-------\u20ac`,
			success: true,
			want:    []byte(string(`---------9---------9-------`) + string([]byte{0xe2, 0x82, 0xac})),
		},
		{
			name:    "uni1-end-of-ymm-word-single-pass-two-beyond",
			str:     `---------9---------9--------\u20ac`,
			success: true,
			want:    []byte(string(`---------9---------9--------`) + string([]byte{0xe2, 0x82, 0xac})),
		},
		{
			name:    "uni1-end-of-ymm-word-single-pass-three-beyond",
			str:     `---------9---------9---------\u20ac`,
			success: true,
			want:    []byte(string(`---------9---------9---------`) + string([]byte{0xe2, 0x82, 0xac})),
		},
		{
			name:    "uni1-end-of-ymm-word-single-fail-one-beyond",
			str:     `---------9---------9-------\u20a`,
			success: false,
		},
		{
			name:    "uni1-end-of-ymm-word-single-fail-two-beyond",
			str:     `---------9---------9--------\u20a`,
			success: false,
		},
		{
			name:    "uni1-end-of-ymm-word-single-fail-three-beyond",
			str:     `---------9---------9---------\u20a`,
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// enclose test string in quotes (as validated by stage 1)
			buf := []byte(fmt.Sprintf(`"%s"`, tt.str))

			dst_length := uint64(0)
			need_copy := false
			success := parse_string_simd_validate_only(buf, &dst_length, &need_copy)

			if success != tt.success {
				t.Errorf("TestParseString() got = %v, want %v", success, tt.success)
			}
			if success && !need_copy {
				fmt.Println(dst_length)
				if dst_length != uint64(len(tt.want)) {
					t.Errorf("TestParseString() got = %d, want %d", dst_length, len(tt.want))
				}
			}
		})
	}
}
