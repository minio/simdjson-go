package main

import (
	"fmt"
	"log"

	"github.com/minio/simdjson-go"
)

func main() {
	// Parse JSON:
	pj, err := simdjson.Parse([]byte(`{
	"key": "val",
	"list": [{
		"key1": "{\"key2\": \"val1\"}"
	}]
}`), nil)
	if err != nil {
		log.Fatal(err)
	}

	iter := pj.Iter()
	list, err := iter.FindElement(nil, "list")
	if err != nil {
		log.Fatal(err)
	}
	arr, err := list.Iter.Array(nil)
	if err != nil {
		log.Fatal(err)
	}
	arr.ForEach(func(i simdjson.Iter) {
		if i.Type() != simdjson.TypeObject {
			return
		}
		elem, err := i.FindElement(nil, `key1`)
		if err != nil {
			return
		}
		fmt.Println(elem.Iter.String())
	})
	// Output:
	// {"key2": "val1"} <nil>
}
