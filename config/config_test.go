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
		RemoteEndpoint: "http://test.default.svc.cluster.local",
		ResourcesWatch: []string{
			"namespaces",
			"pods",
			"deployments",
		},
	}
	testConf, err := ReadConfig("test_config.yaml")
	if err != nil {
		t.Errorf(err.Error())
	}

	if goodConfig.RemoteEndpoint != testConf.RemoteEndpoint {
		t.Errorf("RemoteEndpoint Configurations do not match, got: %v, want: %v.",
			testConf.RemoteEndpoint, goodConfig.RemoteEndpoint)
	}

	sort.Strings(goodConfig.ResourcesWatch)
	sort.Strings(testConf.ResourcesWatch)
	for i := 0; i < len(goodConfig.ResourcesWatch); i++ {
		if goodConfig.ResourcesWatch[i] != testConf.ResourcesWatch[i] {
			t.Errorf("ResourcesWatch Configurations do not match, got: %v, want: %v.",
				testConf.ResourcesWatch[i], goodConfig.ResourcesWatch[i])
		}
	}

}

func TestDiffConfig(t *testing.T) {
	oldRes := []string{"namespaces", "pods"}
	newRes := []string{"deployments", "pods"}
	newResWatch := []string{"deployments", "pods"}
	testConfig := Config{}

	testConfig.DiffConfig(oldRes, newRes)
	for i := 0; i < len(testConfig.NewResources); i++ {
		if testConfig.NewResources[i] != newRes[i] {
			t.Errorf("NewResources Configurations do not match, got: %v, want: %v.",
				testConfig.NewResources[i], newRes[i])
		}
	}

	for i := 0; i < len(testConfig.StaleResources); i++ {
		if testConfig.StaleResources[i] != "namespaces" {
			t.Errorf("StaleResources Configurations do not match, got: %v, want: %v.",
				testConfig.StaleResources[i], "namespaces")
		}
	}

	sort.Strings(testConfig.ResourcesWatch)
	for i := 0; i < len(testConfig.ResourcesWatch); i++ {
		if testConfig.ResourcesWatch[i] != newResWatch[i] {
			t.Errorf("ResourcesWatch Configurations do not match, got: %v, want: %v.",
				testConfig.ResourcesWatch[i], newResWatch[i])
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
