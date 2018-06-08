// Copyright 2018 Heptio
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package emitter

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/glog"
)

func TestEmitChanges(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contents, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Test HTTP Server had error reading body")
		}
		glog.Infof("%s\n", string(contents))
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
