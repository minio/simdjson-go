package simdjson

import (
	"encoding/binary"
	"fmt"
	"math"
)

const JSONVALUEMASK = 0xffffffffffffff
const DEFAULTMAXDEPTH = 128

type ParsedJson struct {
	structural_indexes      []uint32
	tape                    []uint64
	containing_scope_offset []uint64
	isvalid                 bool
	strings                 []byte
}

func (pj *ParsedJson) initialize(size int) {
	pj.tape = make([]uint64, 0, size)
	pj.strings = make([]byte, 0, size)
	pj.structural_indexes = make([]uint32, 0, size)

	// combine into single struct (array)
	pj.containing_scope_offset = make([]uint64, 0, DEFAULTMAXDEPTH)
}

func (pj *ParsedJson) get_current_loc() uint64 {
	return uint64(len(pj.tape))
}

func (pj *ParsedJson) write_tape(val uint64, c byte) {
	pj.tape = append(pj.tape, val|(uint64(c)<<56))
}

func (pj *ParsedJson) write_tape_s64(val int64) {
	pj.write_tape(0, 'l')
	pj.tape = append(pj.tape, uint64(val))
}

func (pj *ParsedJson) write_tape_double(d float64) {
	pj.write_tape(0, 'd')
	pj.tape = append(pj.tape, math.Float64bits(d))
}

func (pj *ParsedJson) annotate_previousloc(saved_loc uint64, val uint64) {
	pj.tape[saved_loc] |= val
}

func (pj *ParsedJson) dump_raw_tape() bool {

	if !pj.isvalid {
		return false
	}

	tapeidx := uint64(0)
	howmany := uint64(0)
	tape_val := pj.tape[tapeidx]
	ntype := tape_val >> 56
	fmt.Printf("%d : %s", tapeidx, string(ntype))

	if ntype == 'r' {
		howmany = tape_val & JSONVALUEMASK
	} else {
		fmt.Errorf("Error: no starting root node?\n")
		return false
	}
	fmt.Printf("\t// pointing to %d (right after last node)\n", howmany)

	tapeidx++
	for ; tapeidx < howmany; tapeidx++ {
		tape_val = pj.tape[tapeidx];
		fmt.Printf("%d : ", tapeidx)
		ntype := tape_val >> 56
		payload := tape_val & JSONVALUEMASK
		switch ntype {
		case '"': // we have a string
			fmt.Printf("string \"")
			string_length := uint64(binary.LittleEndian.Uint32(pj.strings[payload : payload+4]))
			fmt.Printf("%s", print_with_escapes(pj.strings[payload+4:payload+4+string_length]))
			fmt.Println("\"")

		case 'l': // we have a long int
			if tapeidx+1 >= howmany {
				return false
			}
			tapeidx++
			fmt.Printf("integer %d\n", int64(pj.tape[tapeidx]))

		case 'd': // we have a double
			if tapeidx+1 >= howmany {
				return false
			}
			tapeidx++
			fmt.Printf("float %f\n", math.Float64frombits(pj.tape[tapeidx]))

		case 'n': // we have a null
			fmt.Printf("null\n")

		case 't': // we have a true
			fmt.Printf("true\n")

		case 'f': // we have a false
			fmt.Printf("false\n")

		case '{': // we have an object
			fmt.Printf("{\t// pointing to next tape location %d (first node after the scope) \n", payload)

		case '}': // we end an object
			fmt.Printf("}\t// pointing to previous tape location %d (start of the scope) \n", payload)

		case '[': // we start an array
			fmt.Printf("\t// pointing to next tape location %d (first node after the scope) \n", payload)

		case ']': // we end an array
			fmt.Printf("]\t// pointing to previous tape location %d (start of the scope) \n", payload)

		case 'r': // we start and end with the root node
			fmt.Printf("end of root\n")
			return false

		default:
			return false
		}
	}

	tape_val = pj.tape[tapeidx]
	payload := tape_val & JSONVALUEMASK
	ntype = tape_val >> 56
	fmt.Printf("%d : %s\t// pointing to %d (start root)\n", tapeidx, string(ntype), payload)

	return true
}

func print_with_escapes(src []byte) string {

	result := make([]byte, 0, len(src))

	for _, s := range src {
		switch s {
		case '\b':
			result = append(result, []byte{'\\', 'b'}...)

		case '\f':
			result = append(result, []byte{'\\', 'f'}...)

		case '\n':
			result = append(result, []byte{'\\', 'n'}...)

		case '\r':
			result = append(result, []byte{'\\', 'r'}...)

		case '"':
			result = append(result, []byte{'\\', '"'}...)

		case '\t':
			result = append(result, []byte{'\\', 't'}...)

		case '\\':
			result = append(result, []byte{'\\', '\\'}...)

		default:
			if s <= 0x1f {
				result = append(result, []byte(fmt.Sprintf("%04x", s))...)
			} else {
				result = append(result, s)
			}
		}
	}

	return string(result)
}
