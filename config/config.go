// Copyright 2018-2019 VMware, Inc. 
// SPDX-License-Identifier: Apache-2.0 

package config

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	yaml "gopkg.in/yaml.v2"
)

type FieldPruner map[string][]string

type Config struct {
	Endpoints             []RemoteEndpoint `yaml:"remoteEndpoints"`
	ResourcesWatch        []Resource       `yaml:"resources"`
	NewResources          []Resource
	StaleResources        []Resource
	PrometheusMetrics     PrometheusConfig       `yaml:"prometheusMetrics"`
	ClusterName           string                 `yaml:"clusterName"`
	DeltaUpdates          bool                   `yaml:"deltaUpdates"`
	DelayStartSeconds     string                 `yaml:"delayAddEventDuration"`
	EmitCacheDuration     string                 `yaml:"emitCacheDuration"`
	EmitBatchMaxObjects   int                    `yaml:"emitBatchMaxObjects"`
	EmitInterval          int                    `yaml:"emitInterval"`
	ForceReuploadDuration string                 `yaml:"forceReuploadDuration"`
	Metadata              map[string]interface{} `yaml:"metadata"`
	HttpLiveness          HttpLivenessConfig     `yaml:"httpLiveness"`
	IgnoreNamespaces      []string               `yaml:"ignoreNamespaces"`
}

type Resource struct {
	Name         string   `yaml:"name"`
	AssetId      string   `yaml:"assetId"`
	PruneFields  []string `yaml:"pruneFields"`
	FilterEvents []string `yaml:"filterEvents"`
}

type RemoteEndpoint struct {
	Type          string   `yaml:"type"`
	Region        string   `yaml:"region"`
	Url           string   `yaml:"url"`
	Namespaces    []string `yaml:"namespaces"`
	ResourceTypes []string `yaml:"resourceTypes"`
	UsernameVar   string   `yaml:"usernameVar"`
	PasswordVar   string   `yaml:"passwordVar"`
}

type PrometheusConfig struct {
	Port string `yaml:"port"`
	Path string `yaml:"path"`
}

type HttpLivenessConfig struct {
	Port string `yaml:"port"`
	Path string `yaml:"path"`
}

// ReadConfig reads info from a config file based on the passed configPath.
// If there's an issue locating, reading, or parsing the config, an error is
// returned.
func ReadConfig(configPath string) (*Config, error) {
	fileBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(fileBytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) DiffConfig(old, new []Resource) {
	// TODO(joshrosso): need to determine sorting peice
	// sort.Strings(old)
	// sort.Strings(new)

	keeps := []Resource{}
	drops := []Resource{}
	// make a copy of the data that we can remove from
	// while we iterate on the original
	// the modified news var will be what we build our final list from
	news := new

	for _, oRes := range old {
		match := false
		for i, nRes := range new {
			if oRes.Name == nRes.Name {
				keeps = append(keeps, oRes)
				news = append(news[:i], news[i+1:]...)
				// if we found a match mark the value as match = true
				// and break the inner for loop
				match = true
				break
			}
		}

		// if we got out of the inner for loop and we do not have match
		// add the value to the drops slice
		if match != true {
			drops = append(drops, oRes)
		}
	}

	c.NewResources = news
	c.StaleResources = drops
	c.ResourcesWatch = append(keeps, news...)
}

// New creates new file watcher
func NewFileWatcher(ch chan bool, file string) {
	fileWatcher(ch, file)
}

func fileWatcher(changed chan bool, file string) {
	go func() {
		// create start time of now so it always loads the first time
		startTime := time.Now()
		for {
			info, err := os.Stat(file)
			if err != nil {
				glog.Errorf("error accessing file. error: %s", err)
			}

			modTime := info.ModTime()
			if startTime != modTime {
				// reset the start time to our newly seen modified time
				startTime = modTime
				changed <- true
			}
			// sleep for some time we we aren't always banging
			// on the file system
			time.Sleep(time.Second * 10)
		}
	}()

}
