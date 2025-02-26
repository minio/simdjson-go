package main

import (
	"fmt"
	"log"

	"github.com/minio/simdjson-go"
)

func main() {
	// Parse JSON and print all "scores" as single item.
	pj, err := simdjson.Parse([]byte(`{
    "items": [
        {
            "name": "jim",
            "scores": [
                {
                    "game":"golf",
                    "scores": ["one","two"]
                },
                {
                    "game":"badminton",
                    "scores":["zero",1,"six"]
                },
                [
		            {
		              "game": "nested for some reason?",
                      "scores": ["five"]
                    }
                ]
            ]
        }
    ]
}`), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Iterate each top level element.
	//traverseKeys := map[string]struct{}{"items": {}}
	var parse func(i simdjson.Iter) error
	parse = func(i simdjson.Iter) error {
		switch i.Type() {
		case simdjson.TypeArray:
			array, err := i.Array(nil)
			if err != nil {
				return err
			}
			array.ForEach(func(i simdjson.Iter) {
				parse(i)
			})
		case simdjson.TypeObject:
			obj, err := i.Object(nil)
			if err != nil {
				return err
			}
			scores := obj.FindKey("scores", nil)
			if scores == nil || scores.Type != simdjson.TypeArray {
				return obj.ForEach(func(_ []byte, i simdjson.Iter) {
					parse(i)
				}, nil)
			}
			array, err := scores.Iter.Array(nil)
			if err != nil {
				return err
			}
			array.ForEach(func(i simdjson.Iter) {
				switch i.Type() {
				case simdjson.TypeString, simdjson.TypeInt:
					s, _ := i.StringCvt()
					fmt.Println("Found score:", s)
				case simdjson.TypeObject, simdjson.TypeArray:
					parse(i)
				}
			})
		default:
			fmt.Println("Ignoring", i.Type())
		}

		return nil
	}
	_ = pj.ForEach(parse)

	// Output:
	//Found score: one
	//Found score: two
	//Found score: zero
	//Found score: 1
	//Found score: six
	//Found score: five
}
