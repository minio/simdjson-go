package simdjson

import (
	"fmt"
	"log"
	"testing"
)

func TestObject_FindPath(t *testing.T) {
	tests := []struct {
		name     string
		path     []string
		wantName string
		wantType Type
		wantVal  string
		wantErr  bool
	}{
		{
			name:     "top",
			path:     []string{"Alt"},
			wantName: "Alt",
			wantType: TypeString,
			wantVal:  `"Image of city"`,
		},
		{
			name:     "nested-1",
			path:     []string{"Image", "Animated"},
			wantName: "Animated",
			wantType: TypeBool,
			wantVal:  "false",
		},
		{
			name:     "nested-2",
			path:     []string{"Image", "Thumbnail", "Url"},
			wantName: "Url",
			wantType: TypeString,
			wantVal:  `"http://www.example.com/image/481989943"`,
		},
		{
			name:     "int",
			path:     []string{"Image", "Height"},
			wantName: "Height",
			wantType: TypeInt,
			wantVal:  `600`,
		},
		{
			name:     "obj",
			path:     []string{"Image", "Thumbnail"},
			wantName: "Thumbnail",
			wantType: TypeObject,
			wantVal:  `{"Height":125,"Url":"http://www.example.com/image/481989943","Width":100}`,
		},
		{
			name:     "array",
			path:     []string{"Image", "IDs"},
			wantName: "IDs",
			wantType: TypeArray,
			wantVal:  `[116,943,234,38793]`,
		},
		{
			name:    "404",
			path:    []string{"Image", "NonEx"},
			wantErr: true,
		},
	}
	input := `{
    "Image":
    {
        "Animated": false,
        "Height": 600,
        "IDs":
        [
            116,
            943,
            234,
            38793
        ],
        "Thumbnail":
        {
            "Height": 125,
            "Url": "http://www.example.com/image/481989943",
            "Width": 100
        },
        "Title": "View from 15th Floor",
        "Width": 800
    },
	"Alt": "Image of city" 
}`
	for _, tt := range tests {
		pj, err := Parse([]byte(input), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(tt.name, func(t *testing.T) {
			i := pj.Iter()
			i.AdvanceInto()
			_, root, err := i.Root(nil)
			if err != nil {
				t.Fatal(err)
			}
			obj, err := root.Object(nil)
			if err != nil {
				t.Fatal(err)
			}

			elem, err := obj.FindPath(nil, tt.path...)
			if err != nil && !tt.wantErr {
				t.Fatal(err)
			}
			if tt.wantErr {
				return
			}
			if elem.Type != tt.wantType {
				t.Errorf("Want type %v, got %v", tt.wantType, elem.Type)
			}
			if elem.Name != tt.wantName {
				t.Errorf("Want name %v, got %v", tt.wantName, elem.Name)
			}
			ser, err := elem.Iter.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(ser) != tt.wantVal {
				t.Errorf("want '%s', got '%s'", tt.wantVal, string(ser))
			}
		})
	}
}

func ExampleObject_FindPath() {
	input := `{
    "Image":
    {
        "Animated": false,
        "Height": 600,
        "IDs":
        [
            116,
            943,
            234,
            38793
        ],
        "Thumbnail":
        {
            "Height": 125,
            "Url": "http://www.example.com/image/481989943",
            "Width": 100
        },
        "Title": "View from 15th Floor",
        "Width": 800
    },
	"Alt": "Image of city" 
}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		log.Fatal(err)
	}
	i := pj.Iter()
	i.AdvanceInto()

	// Grab root
	_, root, err := i.Root(nil)
	if err != nil {
		log.Fatal(err)
	}
	// Grab top object
	obj, err := root.Object(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Find element in path.
	elem, err := obj.FindPath(nil, "Image", "Thumbnail", "Url")
	if err != nil {
		log.Fatal(err)
	}

	// Print result:
	fmt.Println(elem.Type)
	fmt.Println(elem.Iter.String())

	// Output:
	// string
	// http://www.example.com/image/481989943 <nil>
}
