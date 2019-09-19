package simdjson

type ParsedJson struct {
	structural_indexes []uint32
	tape               []byte
	containing_scope_offset []int
	ret_address        []byte
	current_loc		   int
	isvalid			   bool
}

func (pj *ParsedJson) get_current_loc() int {
	return pj.current_loc
}

func (pj *ParsedJson) write_tape(pos int, c byte) {
	pj.tape[pos] = c
}

func (pj *ParsedJson) annotate_previousloc(a int, b int) {

}

