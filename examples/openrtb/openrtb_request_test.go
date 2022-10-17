package main

import (
	"testing"
)

func Benchmark_populateTagParams(t *testing.B) {
	for i := 0; i < t.N; i++ {
		tagParams, err := populateTagParams(reqBody)
		if err != nil {
			t.Errorf("populateTagParams() err %s", err)
			return
		}
		if tagParams.ReqId != "banner1" {
			t.Errorf("populateTagParams() ReqId %s is not banner1", tagParams.ReqId)
		}
		ReleaseTagParams(tagParams)
	}
}
