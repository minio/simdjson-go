package simdjson

import (
	"testing"
)

func TestParseND(t *testing.T) {
	type args struct {
		contents string
	}
	tests := []struct {
		name    string
		js      string
		want    string
		wantErr bool
	}{
		{
			name: "demo",
			js: `{"three":true,"two":"foo","one":-1}
{"three":false,"two":"bar","one":null}
{"three":true,"two":"baz","one":2.5}`,
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseND([]byte(tt.js), nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseND() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			//if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("ParseND() got = %v, want %v", got, tt.want)
			//}
			i := got.Iter()
			for i.Advance() == TypeRoot {
				obj, _ := i.Root(nil)
				b, err := obj.MarshalJSON()
				t.Log("err:", err, "json:", string(b))
			}
			t.Log(int(i.PeekNextTag()))
		})
	}
}
