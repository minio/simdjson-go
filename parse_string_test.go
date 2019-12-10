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
