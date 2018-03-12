package config

import (
	"io/ioutil"
	"log"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	RemoteEndpoint string   `yaml:"remoteEndpoint"`
	ResourceWatch  []string `yaml:"resources"`
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
