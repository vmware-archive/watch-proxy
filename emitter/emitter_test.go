package emitter

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmitChanges(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contents, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Test HTTP Server had error reading body")
		}
		log.Printf("%s\n", string(contents))
		if string(contents) != "\"{foo: bar, bar: baz}\"" {
			t.Errorf("Request body is wrong, got: %v, want: %v.",
				string(contents), "\"{foo: bar, bar: baz}\"")
		}
	}))

	type args struct {
		newData interface{}
		url     string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "bar",
			args: args{
				newData: "{foo: bar, bar: baz}",
				url:     ts.URL,
			},
		},
	}

	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			EmitChanges(tt.args.newData, tt.args.url)
		})
	}
}
