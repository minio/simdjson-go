//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/minio/simdjson-go"
)

func printKeyHistogram(pj *simdjson.ParsedJson, key string) (err error) {
	var elem *simdjson.Element
	count := make(map[string]int)
	err = pj.ForEach(func(i simdjson.Iter) error {
		if elem, err = i.FindElement(elem, key); err != nil {
			return nil
		}
		if elem.Type == simdjson.TypeString {
			s, _ := elem.Iter.String()
			count[s]++
		}
		return nil
	})
	res, _ := json.Marshal(count)
	fmt.Println(key, ":", string(res)+"\n")
	return err
}

func main() {
	if !simdjson.SupportedCPU() {
		log.Fatal("Unsupported CPU")
	}
	msg, err := os.ReadFile("parking-citations.json")
	if err != nil {
		log.Fatalf("Failed to load file: %v", err)
	}

	parsed, err := simdjson.ParseND(msg, nil)
	if err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	printKeyHistogram(parsed, "Make")
	printKeyHistogram(parsed, "MeterId")
	printKeyHistogram(parsed, "ViolationCode")
}
