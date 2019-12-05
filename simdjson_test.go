package simdjson

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseND(t *testing.T) {
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
			want: `{"three":true,"two":"foo","one":-1}
{"three":false,"two":"bar","one":null}
{"three":true,"two":"baz","one":2.5}`,
			wantErr: false,
		},
		{
			name:    "valid",
			js:      `{"bimbam":12345465.447,"bumbum":true,"istrue":true,"isfalse":false,"aap":null}`,
			want:    `{"bimbam":12345465.447,"bumbum":true,"istrue":true,"isfalse":false,"aap":null}`,
			wantErr: false,
		},
		{
			name:    "floatinvalid",
			js:      `{"bimbam":12345465.44j7,"bumbum":true}`,
			wantErr: true,
		},
		{
			name:    "numberinvalid",
			js:      `{"bimbam":1234546544j7}`,
			wantErr: true,
		},
		{
			name:    "emptyobject",
			js:      `{}`,
			want:    `{}`,
			wantErr: false,
		},
		{
			name:    "emptyslice",
			js:      ``,
			wantErr: true,
		},
		{
			name: "types",
			js: `{"controversiality":0,"body":"A look at Vietnam and Mexico exposes the myth of market liberalisation.","subreddit_id":"t5_6","link_id":"t3_17863","stickied":false,"subreddit":"reddit.com","score":2,"ups":2,"author_flair_css_class":null,"created_utc":1134365188,"author_flair_text":null,"author":"frjo","id":"c13","edited":false,"parent_id":"t3_17863","gilded":0,"distinguished":null,"retrieved_on":1473738411}
{"created_utc":1134365725,"author_flair_css_class":null,"score":1,"ups":1,"subreddit":"reddit.com","stickied":false,"link_id":"t3_17866","subreddit_id":"t5_6","controversiality":0,"body":"The site states \"What can I use it for? Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more...\", just like any other new breeed of sites that want us to store everything we have on the web. And they even guarantee multiple levels of security and encryption etc. But what prevents these web site operators fom accessing and/or stealing Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more, for competitive or personal gains...? I am pretty sure that most of them are honest, but what's there to prevent me from setting up a good useful site and stealing all your data? Call me paranoid - I am.","retrieved_on":1473738411,"distinguished":null,"gilded":0,"id":"c14","edited":false,"parent_id":"t3_17866","author":"zse7zse","author_flair_text":null}`,
			want: `{"controversiality":0,"body":"A look at Vietnam and Mexico exposes the myth of market liberalisation.","subreddit_id":"t5_6","link_id":"t3_17863","stickied":false,"subreddit":"reddit.com","score":2,"ups":2,"author_flair_css_class":null,"created_utc":1134365188,"author_flair_text":null,"author":"frjo","id":"c13","edited":false,"parent_id":"t3_17863","gilded":0,"distinguished":null,"retrieved_on":1473738411}
{"created_utc":1134365725,"author_flair_css_class":null,"score":1,"ups":1,"subreddit":"reddit.com","stickied":false,"link_id":"t3_17866","subreddit_id":"t5_6","controversiality":0,"body":"The site states \"What can I use it for? Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more...\", just like any other new breeed of sites that want us to store everything we have on the web. And they even guarantee multiple levels of security and encryption etc. But what prevents these web site operators fom accessing and/or stealing Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more, for competitive or personal gains...? I am pretty sure that most of them are honest, but what's there to prevent me from setting up a good useful site and stealing all your data? Call me paranoid - I am.","retrieved_on":1473738411,"distinguished":null,"gilded":0,"id":"c14","edited":false,"parent_id":"t3_17866","author":"zse7zse","author_flair_text":null}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseND([]byte(tt.js), nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseND() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			// Compare all
			i := got.Iter()
			b2, err := i.MarshalJSON()
			if string(b2) != tt.want {
				t.Errorf("ParseND() got = %v, want %v", string(b2), tt.want)
			}

			// Compare each element
			i = got.Iter()
			ref := strings.Split(tt.js, "\n")
			for i.Advance() == TypeRoot {
				_, obj, err := i.Root(nil)
				if err != nil {
					t.Fatal(err)
				}
				want := ref[0]
				ref = ref[1:]
				got, err := obj.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("ParseND() got = %v, want %v", string(got), want)
				}
			}

			i = got.Iter()
			ref = strings.Split(tt.js, "\n")
			for i.Advance() == TypeRoot {
				typ, obj, err := i.Root(nil)
				if err != nil {
					t.Fatal(err)
				}
				switch typ {
				case TypeObject:
					// We must send it throught marshall/unmarshall to match.
					var want = ref[0]
					var tmpMap map[string]interface{}
					err := json.Unmarshal([]byte(want), &tmpMap)
					if err != nil {
						t.Fatal(err)
					}
					w2, err := json.Marshal(tmpMap)
					if err != nil {
						t.Fatal(err)
					}
					want = string(w2)
					got, err := obj.Interface()
					if err != nil {
						t.Fatal(err)
					}
					gotAsJson, err := json.Marshal(got)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(string(gotAsJson), want) {
						t.Errorf("ParseND() got = %#v, want %#v", string(gotAsJson), want)
					}
				}
				ref = ref[1:]
			}
		})
	}
}

func TestParseFailCases(t *testing.T) {
	tests := []struct {
		name    string
		js      string
		want    string
		wantErr bool
	}{
		{
			name:    "fail01_EXCLUDE",
			js:      `"A JSON payload should be an object or array, not a string."`,
			wantErr: true,
		},
		{
			name:    "fail02",
			js:      `["Unclosed array"`,
			wantErr: true,
		},
		{
			name:    "fail03",
			js:      `{unquoted_key: "keys must be quoted"}`,
			wantErr: true,
		},
		{
			name:    "fail04",
			js:      `["extra comma",]`,
			wantErr: true,
		},
		{
			name:    "fail05",
			js:      `["double extra comma",,]`,
			wantErr: true,
		},
		{
			name:    "fail06",
			js:      `[   , "<-- missing value"]`,
			wantErr: true,
		},
		{
			name:    "fail07",
			js:      `["Comma after the close"],`,
			wantErr: true,
		},
		{
			name:    "fail08",
			js:      `["Extra close"]]`,
			wantErr: true,
		},
		{
			name:    "fail09",
			js:      `{"Extra comma": true,}`,
			wantErr: true,
		},
		{
			name:    "fail10",
			js:      `{"Extra value after close": true} "misplaced quoted value"`,
			wantErr: true,
		},
		{
			name:    "fail11",
			js:      `{"Illegal expression": 1 + 2}`,
			wantErr: true,
		},
		{
			name:    "fail12",
			js:      `{"Illegal invocation": alert()}`,
			wantErr: true,
		},
		{
			name:    "fail13",
			js:      `{"Numbers cannot have leading zeroes": 013}`,
			wantErr: true,
		},
		{
			name:    "fail14",
			js:      `{"Numbers cannot be hex": 0x14}`,
			wantErr: true,
		},
		{
			name:    "fail15",
			js:      `["Illegal backslash escape: ` + string(byte(0x15)) + `"]`,
			wantErr: true,
		},
		{
			name:    "fail16",
			js:      `[\naked]`,
			wantErr: true,
		},
		{
			name:    "fail17",
			js:      `["Illegal backslash escape: ` + string(byte(0x17)) + `"]`,
			wantErr: true,
		},
		{
			name:    "fail18",
			js:      `[[[[[[[[[[[[[[[[[[[["Too deep"]]]]]]]]]]]]]]]]]]]]`,
			wantErr: true,
		},
		{
			name:    "fail19",
			js:      `{"Missing colon" null}`,
			wantErr: true,
		},
		{
			name:    "fail20",
			js:      `{"Double colon":: null}`,
			wantErr: true,
		},
		{
			name:    "fail21",
			js:      `{"Comma instead of colon", null}`,
			wantErr: true,
		},
		{
			name:    "fail22",
			js:      `["Colon instead of comma": false]`,
			wantErr: true,
		},
		{
			name:    "fail23",
			js:      `["Bad value", truth]`,
			wantErr: true,
		},
		{
			name:    "fail24",
			js:      `['single quote']`,
			wantErr: true,
		},
		{
			name: "fail25",
			js: `["	tab	character	in	string	"]`,
			wantErr: true,
		},
		// fail26 is disabled for simdjson-go (not leading to errors, C specific escaping)
		//{
		//	name:    "fail26",
		//	js:      `["tab\   character\   in\  string\  "]`,
		//	wantErr: true,
		//},
		{
			name: "fail27",
			js: `["line
break"]`,
			wantErr: true,
		},
		{
			name: "fail28",
			js: `["line\
break"]`,
			wantErr: true,
		},
		{
			name:    "fail29",
			js:      `[0e]`,
			wantErr: true,
		},
		{
			name:    "fail30",
			js:      `[0e+]`,
			wantErr: true,
		},
		{
			name:    "fail31",
			js:      `[0e+-1]`,
			wantErr: true,
		},
		{
			name:    "fail32",
			js:      `{"Comma instead if closing brace": true,`,
			wantErr: true,
		},
		{
			name:    "fail33",
			js:      `["mismatch"}`,
			wantErr: true,
		},
		{
			name: "fail34",
			// `["this string contains bad UTF-8 €"]`
			js:      string([]byte{0x5b, 0x22, 0x74, 0x68, 0x69, 0x73, 0x20, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x20, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x73, 0x20, 0x62, 0x61, 0x64, 0x20, 0x55, 0x54, 0x46, 0x2d, 0x38, 0x20, 0x80, 0x22, 0x5d, 0x0a}),
			wantErr: true,
		},
		{
			name:    "fail35",
			js:      `{"this file" :` + string(byte(0xa0)) + `"has an unbreakable character outside the strings"}`,
			wantErr: true,
		},
		{
			name:    "fail36",
			js:      `["this is an unclosed string ]`,
			wantErr: true,
		},
		{
			name:    "fail37",
			js:      `[12a]`,
			wantErr: true,
		},
		{
			name:    "fail38",
			js:      `[12 a]`,
			wantErr: true,
		},
		{
			name:    "fail39_EXCLUDE",
			js:      `{"name":1,"name":2, "this is allowable as per the json spec": true}`,
			wantErr: false,
		},
		{
			name:    "fail41_toolarge",
			js:      `18446744073709551616`,
			wantErr: true,
		},
		{
			name: "fail42",
			js: `{"fdfds":
"4332" }`,
			wantErr: true,
		},
		{
			name:    "fail43",
			js:      `[-]`,
			wantErr: true,
		},
		{
			name:    "fail44",
			js:      `[-2.]`,
			wantErr: true,
		},
		{
			name:    "fail45",
			js:      `[0.e1]`,
			wantErr: true,
		},
		{
			name:    "fail46",
			js:      `[2.e+3]`,
			wantErr: true,
		},
		{
			name:    "fail47",
			js:      `[2.e-3]`,
			wantErr: true,
		},
		{
			name:    "fail48",
			js:      `[2.e3]`,
			wantErr: true,
		},
		{
			name:    "fail49",
			js:      `[-.123]`,
			wantErr: true,
		},
		{
			name:    "fail50",
			js:      `[1.]`,
			wantErr: true,
		},
		{
			name:    "fail51",
			js:      `[],`,
			wantErr: true,
		},
		{
			name:    "fail52",
			js:      `[x]]`,
			wantErr: true,
		},
		{
			name:    "fail53",
			js:      `{}}`,
			wantErr: true,
		},
		{
			name:    "fail54",
			js:      `[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[(...)`,
			wantErr: true,
		},
		{
			name:    "fail55",
			js:      `[1,]`,
			wantErr: true,
		},
		{
			name:    "fail56",
			js:      `["",]`,
			wantErr: true,
		},
		// fail57 through to fail59 and fail61 are disabled for simdjson-go as Go does not allow illegal unicode chars
		// { name: "fail57", js: `{ "name": "\udc00\ud800\uggggxy" }`, wantErr: true },
		// { name: "fail58", js: `{ "name": "\uc0meatmebro" }`, wantErr: true },
		// { name: "fail59", js: `{ "name": "\uf**k" }`, wantErr: true },
		// { name: "fail61", js: `{"badescape":"\uxhgj"}`, wantErr: true },
		{
			name:    "fail60",
			js:      `[1e+1111]`,
			wantErr: true,
		},
		{
			name:    "fail62",
			js:      `{"foo":"baa}`,
			wantErr: true,
		},
		{
			name:    "fail63",
			js:      `"f[`,
			wantErr: true,
		},
		{
			name:    "fail64",
			js:      `"`,
			wantErr: true,
		},
		{
			name:    "fail65",
			js:      `falsy`,
			wantErr: true,
		},
		{
			name:    "fail66",
			js:      `44`,
			wantErr: true,
		},
		{
			name:    "fail67",
			js:      `4 4`,
			wantErr: true,
		},
		{
			name:    "fail68",
			js:      `04`,
			wantErr: true,
		},
		{
			name:    "fail69",
			js:      `falsefalse`,
			wantErr: true,
		},
		{
			name:    "fail70",
			js:      ``,
			wantErr: true,
		},
		{
			name:    "fail71",
			js:      `"a bad string��"`,
			wantErr: true,
		},
		{
			name:    "fail72",
			js:      `["with bad trailing space" ]`,
			wantErr: true,
		},
		{
			name:    "fail73",
			js:      `10000000000000000000000000000000000000000000e+308`,
			wantErr: true,
		},
		{
			name:    "fail74",
			js:      `[7,7,7,7,6,7,7,7,6,7,7,6,[7,7,7,7,6,7,7,7,6,7,7,6,7,7,7,7,7,7,6`,
			wantErr: true,
		},
		{
			name:    "fail75",
			js:      `f`,
			wantErr: true,
		},
		{
			name:    "noclose",
			js:      `{"bimbam:"something"`,
			wantErr: true,
		},
		{
			name:    "noclose-issue-13",
			js:      `{"000"`,
			wantErr: true,
		},
		{
			name:    "noclose-issue-23",
			js:      `{""0`,
			wantErr: true,
		},
		{
			name:    "issue-17",
			js:      `{"bimbam:12345465.44j7,"bumbum":true}`,
			wantErr: true,
		},
		{
			name: "nonewlineinkeys-issue-27",
			js: `{"
":"","00":""}`,
			wantErr: true,
		},
		{
			name:    "noclose-issue-19",
			js:      `[0.0`,
			wantErr: true,
		},
		{
			name:    "binaryinput-issue-20",
			js:      string([]byte{0x09, 0x20, 0x20, 0x0a}),
			wantErr: true,
		},
		//{
		//	name:    "invalidjson-issue-24",
		//	js:      "{\"\":[],\"\":[5\x00]}",
		//	want:    "{\"\":[],\"\":[5\x00]}",
		//	wantErr: false,
		//},
		//{
		//	name:    "invalidchar-issue-25",
		//	js:      `{"":"\_000"}`,
		//	wantErr: true,
		//},
		{
			name:    "fatal-error-issue-32",
			js:      `{"":`,
			wantErr: true,
		},
		{
			name:    "index-out-of-range-issue-28",
			js:      `[6`,
			wantErr: true,
		},
		{
			name:    "deadlock-issue-29",
			js:      `{""`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.js), nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFailCases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			// Compare all
			i := got.Iter()
			b2, err := i.MarshalJSON()
			if string(b2) != tt.want {
				t.Errorf("ParseFailCases() got = %v, want %v", string(b2), tt.want)
			}

			// Compare each element
			i = got.Iter()
			ref := strings.Split(tt.js, "\n")
			for i.Advance() == TypeRoot {
				_, obj, err := i.Root(nil)
				if err != nil {
					t.Fatal(err)
				}
				want := ref[0]
				ref = ref[1:]
				got, err := obj.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("ParseFailCases() got = %v, want %v", string(got), want)
				}
			}

			i = got.Iter()
			ref = []string{tt.js}
			for i.Advance() == TypeRoot {
				typ, obj, err := i.Root(nil)
				if err != nil {
					t.Fatal(err)
				}
				switch typ {
				case TypeObject:
					// We must send it throught marshall/unmarshall to match.
					var want = ref[0]
					var tmpMap map[string]interface{}
					err := json.Unmarshal([]byte(want), &tmpMap)
					if err != nil {
						t.Fatal(err)
					}
					w2, err := json.Marshal(tmpMap)
					if err != nil {
						t.Fatal(err)
					}
					want = string(w2)
					got, err := obj.Interface()
					if err != nil {
						t.Fatal(err)
					}
					gotAsJson, err := json.Marshal(got)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(string(gotAsJson), want) {
						t.Errorf("ParseFailCases() got = %#v, want %#v", string(gotAsJson), want)
					}
				}
				ref = ref[1:]
			}
		})
	}
}
