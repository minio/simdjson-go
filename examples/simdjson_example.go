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
	printDoubleFine(parsed)
}

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

func printDoubleFine(pj *simdjson.ParsedJson) (err error) {
	var elem *simdjson.Element
	err = pj.ForEach(func(i simdjson.Iter) error {
		if elem, err = i.FindElement(elem, "Make"); err != nil {
			return nil
		}
		make, err := elem.Iter.String()
		if err != nil || make != "BMW" {
			return nil
		}
		if elem, err = i.FindElement(elem, "Fine"); err != nil {
			return nil
		}
		amount, err := elem.Iter.Float()
		if err != nil {
			return nil
		}
		err = elem.Iter.SetFloat(amount * 2)
		if err != nil {
			return err
		}
		return nil
	})
	if true {
		i := pj.Iter()
		res, _ := i.MarshalJSON()
		fmt.Println(string(res))
	}
	return err
}
