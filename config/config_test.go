package config

import (
	"sort"
	"testing"
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
	testConf := ReadConfig("test_config.yaml")

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
