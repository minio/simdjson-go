package simdjson

// ParserOption is a parser option.
type ParserOption func(pj *internalParsedJson) error

// WithCopyStrings will copy strings so they no longer reference the input.
// For enhanced performance, simdjson-go can point back into the original JSON buffer for strings,
// however this can lead to issues in streaming use cases scenarios, or scenarios in which
// the underlying JSON buffer is reused. So the default behaviour is to create copies of all
// strings (not just those transformed anyway for unicode escape characters) into the separate
// Strings buffer (at the expense of using more memory and less performance).
// Default: true - strings are copied.
func WithCopyStrings(b bool) ParserOption {
	return func(pj *internalParsedJson) error {
		pj.copyStrings = b
		return nil
	}
}
