package simdjson

import (
	"fmt"
	"testing"
	"encoding/binary"
)

func TestParseString(t *testing.T) {

	const str = "label"

	buf := make([]byte, 1024)
	copy(buf, fmt.Sprintf(`"%s":`, str))
	stringbuf := make([]byte, 256)

	size := parse_string_simd(buf, &stringbuf)

	// First four bytes are size
	length := int(binary.LittleEndian.Uint32(stringbuf[0:4]))
	if length != len(str) {
		t.Errorf("TestParseString: got: %d want: %d", length, len(str))
	}
	// Then comes value of string
	if string(stringbuf[4:4+length]) != str {
		t.Errorf("TestParseString: got: %s want: %s", string(stringbuf[4:4+length]), str)
	}
	// Followed by NULL-character
	if stringbuf[4+length] != 0 {
		t.Errorf("TestParseString: got: 0x%x want: 0x0", stringbuf[4+length])
	}

	if size != uintptr(4+length+1) {
		t.Errorf("TestParseString: got: %d want: %d", size, uintptr(4+length+1))
	}
}
