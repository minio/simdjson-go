package simdjson

//go:noescape
func _find_newline_delimiters(raw []byte) (mask uint64)

//go:noescape
func __find_newline_delimiters()
