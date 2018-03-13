package config

import (
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	RemoteEndpoint string   `yaml:"remoteEndpoint"`
	ResourcesWatch []string `yaml:"resources"`
	NewResources   []string
	StaleResources []string
}

// ReadConfig reads info from config file
func ReadConfig(configFile string) Config {
	_, err := os.Stat(configFile)
	if err != nil {
		log.Fatal("Config file is missing: ", configFile)
	}
	file, _ := os.Open(configFile)
	fileBytes, err := ioutil.ReadAll(file)
	var config Config
	err = yaml.Unmarshal(fileBytes, &config)
	if err != nil {
		log.Fatalln("config unmarshaling error:", err)
	}

	return config
}

func (c *Config) DiffConfig(old, new []string) {
	sort.Strings(old)
	sort.Strings(new)

	keeps := []string{}
	drops := []string{}
	// make a copy of the data that we can remove from
	// while we iterate on the original
	// the modified news var will be what we build our final list from
	news := new

	for _, oRes := range old {
		match := false
		for i, nRes := range new {
			if oRes == nRes {
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
				log.Println("Error accessing file", err)
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
