package main

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/minio/simdjson-go"
)

func printKey(iter simdjson.Iter, key string) (err error) {

	obj, tmp, elem := &simdjson.Object{}, &simdjson.Iter{}, simdjson.Element{}

	for {
		typ := iter.Advance()

		switch typ {
		case simdjson.TypeRoot:
			if typ, tmp, err = iter.Root(tmp); err != nil {
				return
			}

			if typ == simdjson.TypeObject {
				if obj, err = tmp.Object(obj); err != nil {
					return
				}

				e := obj.FindKey(key, &elem)
				if e != nil && elem.Type == simdjson.TypeString {
					v, _ := elem.Iter.StringBytes()
					fmt.Println(string(v))
				}
			}

		default:
			return
		}
	}
}

func main() {
	if !simdjson.SupportedCPU() {
		log.Fatal("Unsupported CPU")
	}
	msg, err := ioutil.ReadFile("parking-citations.json")
	if err != nil {
		log.Fatalf("Failed to load file: %v", err)
	}

	parsed, err := simdjson.ParseND(msg, nil)
	if err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	printKey(parsed.Iter(), "Make")
}
