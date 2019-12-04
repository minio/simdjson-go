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
			name:    "noclose",
			js:      `{"bimbam:"something"`,
			wantErr: true,
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
			name:    "issue-17",
			js: `{"bimbam:12345465.44j7,"bumbum":true}`,
			wantErr: true,
		},
		{
			name: "types",
			js: `{"controversiality":0,"body":"A look at Vietnam and Mexico exposes the myth of market liberalisation.","subreddit_id":"t5_6","link_id":"t3_17863","stickied":false,"subreddit":"reddit.com","score":2,"ups":2,"author_flair_css_class":null,"created_utc":1134365188,"author_flair_text":null,"author":"frjo","id":"c13","edited":false,"parent_id":"t3_17863","gilded":0,"distinguished":null,"retrieved_on":1473738411}
{"created_utc":1134365725,"author_flair_css_class":null,"score":1,"ups":1,"subreddit":"reddit.com","stickied":false,"link_id":"t3_17866","subreddit_id":"t5_6","controversiality":0,"body":"The site states \"What can I use it for? Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more...\", just like any other new breeed of sites that want us to store everything we have on the web. And they even guarantee multiple levels of security and encryption etc. But what prevents these web site operators fom accessing and/or stealing Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more, for competitive or personal gains...? I am pretty sure that most of them are honest, but what's there to prevent me from setting up a good useful site and stealing all your data? Call me paranoid - I am.","retrieved_on":1473738411,"distinguished":null,"gilded":0,"id":"c14","edited":false,"parent_id":"t3_17866","author":"zse7zse","author_flair_text":null}
`,
			want: `{"controversiality":0,"body":"A look at Vietnam and Mexico exposes the myth of market liberalisation.","subreddit_id":"t5_6","link_id":"t3_17863","stickied":false,"subreddit":"reddit.com","score":2,"ups":2,"author_flair_css_class":null,"created_utc":1134365188,"author_flair_text":null,"author":"frjo","id":"c13","edited":false,"parent_id":"t3_17863","gilded":0,"distinguished":null,"retrieved_on":1473738411}
{"created_utc":1134365725,"author_flair_css_class":null,"score":1,"ups":1,"subreddit":"reddit.com","stickied":false,"link_id":"t3_17866","subreddit_id":"t5_6","controversiality":0,"body":"The site states \"What can I use it for? Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more...\", just like any other new breeed of sites that want us to store everything we have on the web. And they even guarantee multiple levels of security and encryption etc. But what prevents these web site operators fom accessing and/or stealing Meeting notes, Reports, technical specs Sign-up sheets, proposals and much more, for competitive or personal gains...? I am pretty sure that most of them are honest, but what's there to prevent me from setting up a good useful site and stealing all your data? Call me paranoid - I am.","retrieved_on":1473738411,"distinguished":null,"gilded":0,"id":"c14","edited":false,"parent_id":"t3_17866","author":"zse7zse","author_flair_text":null}
`,
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
