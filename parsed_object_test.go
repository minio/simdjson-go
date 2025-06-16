package simdjson

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

func TestObject_FindPath(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
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

func TestObject_ForEach(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
		"key6": "value6",
		"key7": "value7",
		"key8": "value8",
		"key9": "value9",
		"key10": "value10"
	}`
	tests := []struct {
		name     string
		onlyKeys []string
		want     map[string]string
	}{
		{
			name:     "all-keys",
			onlyKeys: nil,
			want: map[string]string{
				"key1":  "value1",
				"key2":  "value2",
				"key3":  "value3",
				"key4":  "value4",
				"key5":  "value5",
				"key6":  "value6",
				"key7":  "value7",
				"key8":  "value8",
				"key9":  "value9",
				"key10": "value10",
			},
		},
		{
			name:     "some-keys",
			onlyKeys: []string{"key1", "key3"},
			want: map[string]string{
				"key1": "value1",
				"key3": "value3"},
		},
		{
			name: "sparse-keys",
			onlyKeys: []string{
				"key1", "key3", "key5", "key7", "key9"},
			want: map[string]string{
				"key1": "value1",
				"key3": "value3",
				"key5": "value5",
				"key7": "value7",
				"key9": "value9",
			},
		},
		{
			name:     "sparse-keys",
			onlyKeys: []string{"key1", "key2", "key3", "key9", "key10"},
			want: map[string]string{
				"key1":  "value1",
				"key2":  "value2",
				"key3":  "value3",
				"key9":  "value9",
				"key10": "value10",
			},
		},
		{
			name:     "no-keys",
			onlyKeys: []string{"key20"},
			want:     map[string]string{},
		},
	}
	for _, tt := range tests {
		pj, err := Parse([]byte(input), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(tt.name, func(t *testing.T) {
			filtered := make(map[string]string)
			var onlyKeys map[string]struct{}
			if len(tt.onlyKeys) > 0 {
				onlyKeys = make(map[string]struct{}, len(tt.onlyKeys))
				for _, key := range tt.onlyKeys {
					onlyKeys[key] = struct{}{}
				}
			}
			i := pj.Iter()
			var elem Iter
			ty, err := i.AdvanceIter(&elem)

			if err != nil || ty != TypeRoot {
				t.Fatal(err)
			}
			_ = elem.AdvanceInto()
			obj, err := elem.Object(nil)
			if err != nil {
				t.Fatal(err)
			}
			err = obj.ForEach(func(key []byte, i Iter) {
				value, _ := i.StringCvt()
				filtered[string(key)] = value
			}, onlyKeys)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tt.want, filtered) {
				t.Errorf("want %v\ngot %v\n", tt.want, filtered)
			}
		})
	}
}

func ExampleObject_FindPath() {
	if !SupportedCPU() {
		// Fake it
		fmt.Println("string\nhttp://www.example.com/image/481989943 <nil>")
		return
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

func ExampleArray() {
	if !SupportedCPU() {
		// Fake it
		fmt.Println("Found array\nType: int value: 116\nType: int value: 943\nType: int value: 234\nType: int value: 38793")
		return
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
	elem, err := obj.FindPath(nil, "Image", "IDs")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Found", elem.Type)
	if elem.Type == TypeArray {
		array, err := elem.Iter.Array(nil)
		if err != nil {
			log.Fatal(err)
		}
		array.ForEach(func(i Iter) {
			asString, _ := i.StringCvt()
			fmt.Println("Type:", i.Type(), "value:", asString)
		})
	}
	//Output:
	//Found array
	//Type: int value: 116
	//Type: int value: 943
	//Type: int value: 234
	//Type: int value: 38793
}

func ExampleArray_DeleteElems() {
	if !SupportedCPU() {
		// Fake it
		fmt.Println("Found array\nModified: {\"Image\":{\"Animated\":false,\"Height\":600,\"IDs\":[943,38793]},\"Alt\":\"Image of city\"}")
		return
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
        ]
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
	elem, err := obj.FindPath(nil, "Image", "IDs")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Found", elem.Type)
	if elem.Type == TypeArray {
		array, err := elem.Iter.Array(nil)
		if err != nil {
			log.Fatal(err)
		}
		// Delete all integer elements that are < 500
		array.DeleteElems(func(i Iter) bool {
			if id, err := i.Int(); err == nil {
				return id < 500
			}
			return false
		})
	}
	b, err := root.MarshalJSON()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Modified:", string(b))
	//Output:
	//Found array
	//Modified: {"Image":{"Animated":false,"Height":600,"IDs":[943,38793]},"Alt":"Image of city"}
}
