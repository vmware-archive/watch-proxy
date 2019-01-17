// Copyright 2018 Heptio
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"io/ioutil"
	"os"
	"sort"
	"testing"
	"time"
)

func TestReadConfig(t *testing.T) {
	goodConfig := Config{
		Endpoints: []RemoteEndpoint{
			RemoteEndpoint{
				Type: "http",
				Url:  "http://test.default.svc.cluster.local",
			},
		},
		ResourcesWatch: []Resource{
			Resource{
				Name: "namespaces",
			},
			Resource{
				Name: "pods",
			},
			Resource{
				Name: "deployments",
			},
		},
	}
	testConf, err := ReadConfig("test_config.yaml")
	if err != nil {
		t.Errorf(err.Error())
	}

	if goodConfig.Endpoints[0].Url != testConf.Endpoints[0].Url {
		t.Errorf("RemoteEndpoint Configurations do not match, got: %v, want: %v.",
			testConf.Endpoints[0].Url, goodConfig.Endpoints[0].Url)
	}

	goodConfigResources := []string{}
	testConfigResources := []string{}
	for _, r := range goodConfig.ResourcesWatch {
		goodConfigResources = append(goodConfigResources, r.Name)
	}
	for _, r := range testConf.ResourcesWatch {
		testConfigResources = append(testConfigResources, r.Name)
	}
	sort.Strings(goodConfigResources)
	sort.Strings(testConfigResources)
	for i := 0; i < len(goodConfigResources); i++ {
		if goodConfigResources[i] != testConfigResources[i] {
			t.Errorf("ResourcesWatch Configurations do not match, got: %v, want: %v.",
				testConfigResources[i], goodConfigResources[i])
		}
	}
}

func TestDiffConfig(t *testing.T) {
	oldRes := []Resource{
		Resource{Name: "namespaces"},
		Resource{Name: "pods"},
	}
	newRes := []Resource{
		Resource{Name: "deployments"},
		Resource{Name: "pods"},
	}
	newResWatch := []string{"deployments", "pods"}
	testConfig := Config{}

	testConfig.DiffConfig(oldRes, newRes)
	for i := 0; i < len(testConfig.NewResources); i++ {
		if testConfig.NewResources[i].Name != newRes[i].Name {
			t.Errorf("NewResources Configurations do not match, got: %v, want: %v.",
				testConfig.NewResources[i].Name, newRes[i].Name)
		}
	}

	for i := 0; i < len(testConfig.StaleResources); i++ {
		if testConfig.StaleResources[i].Name != "namespaces" {
			t.Errorf("StaleResources Configurations do not match, got: %v, want: %v.",
				testConfig.StaleResources[i].Name, "namespaces")
		}
	}

	testResWatch := []string{}
	for _, r := range testConfig.ResourcesWatch {
		testResWatch = append(testResWatch, r.Name)
	}
	sort.Strings(testResWatch)
	for i := 0; i < len(testResWatch); i++ {
		if testResWatch[i] != newResWatch[i] {
			t.Errorf("ResourcesWatch Configurations do not match, got: %v, want: %v.",
				testResWatch[i], newResWatch[i])
		}
	}
}

func TestFileWatcher(t *testing.T) {
	fileChange := make(chan bool)
	buf := []byte("foo")
	_ = ioutil.WriteFile("./file.test", buf, 0644)
	fileWatcher(fileChange, "./file.test")
	time.Sleep(time.Second * 3)

	go func() {
	outer:
		for {
			select {
			case _ = <-fileChange:
				break outer
			}
		}
		return
	}()

	buf = []byte("test")
	_ = ioutil.WriteFile("./file.test", buf, 0644)
	defer os.Remove("./file.test")
}
