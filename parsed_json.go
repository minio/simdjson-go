package simdjson

type ParsedJson struct {
	structural_indexes      []uint32
	tape                    []uint64
	containing_scope_offset []uint64
	ret_address             []byte
	isvalid                 bool
	current_string_buf_loc  uint64
	string_buf				uint64
	strings                 []byte
}

func (pj *ParsedJson) get_current_loc() uint64 {
	return uint64(len(pj.tape))
}

func (pj *ParsedJson) write_tape(val uint64, c byte) {
	pj.tape = append(pj.tape, val | (uint64(c) << 56))
}

func (pj *ParsedJson) write_tape_s64(val int64) {
	pj.write_tape(0, 'l')
	pj.tape = append(pj.tape, uint64(val))
}

func (pj *ParsedJson) write_tape_double(d float64) {
	pj.write_tape(0, 'd')
	panic("put memory presentation of float onto tape")
	//memcpy(& tape[current_loc++], &d, sizeof(double));
	pj.tape = append(pj.tape, uint64(123 /*d*/))
}

func (pj *ParsedJson) annotate_previousloc(saved_loc uint64, val uint64) {
	pj.tape[saved_loc] |= val
}
