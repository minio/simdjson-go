package simdjson

//go:noescape
func find_newline_delimiters(raw []byte, indices []uint32, delimiter uint64) (rows uint64)

