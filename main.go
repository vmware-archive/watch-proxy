package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/heptio/quartermaster/cluster"
	"github.com/heptio/quartermaster/config"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var remoteEnd *string

func startWatchers(client *kubernetes.Clientset, config config.Config) map[string]chan bool {
	// loop through list of resources to watch and startup watchers
	// for those resources.
	doneChannels := make(map[string]chan bool)

	resources := []string{}
	// decide if we want to load the full set of resources. Usually at startup
	// or just start up new watchers after config change
	if len(config.NewResources) > 0 {
		resources = config.NewResources
	} else {
		resources = config.ResourcesWatch
	}

	// start the watchers
	for _, resource := range resources {
		switch resource {
		case "namespaces":
			done := make(chan bool)
			doneChannels[resource] = done
			// watch namespaces
			go func() {
				cluster.Namespaces(client, config, done)
			}()
		case "pods":
			// watch pods
			go func() {
				cluster.Pods(client, config)
			}()
		case "deployments":
			// watch deployments
			go func() {
				cluster.Deployments(client, config)
			}()
		}
	}

	return doneChannels
}

func stopWatchers(doneChannels map[string]chan bool, config config.Config) {

	for _, staleRes := range config.StaleResources {
		log.Println("Stopping", staleRes)
		doneChannels[staleRes] <- true
	}
}

func main() {

	configFile := flag.String("c", "/etc/quartermaster/config.yaml", "Path to quartermaster config file")
	flag.Parse()
	qmConfig := config.ReadConfig(*configFile)

	// creates the in-cluster config
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		panic(err.Error())
	}

	// get the cluster version
	// version := cluster.Version(clientset)

	// fire up the watchers
	doneChans := startWatchers(clientset, qmConfig)

	// watch for changes to the config file and
	// reload if the config and adjust watchers if there are changes
	fileChange := make(chan bool)
	fw := config.FileWatcher{}
	fw.New(fileChange, *configFile)
	go func() {
		for {
			select {
			case _ = <-fileChange:
				log.Println("Updating Config")
				newConfig := config.ReadConfig(*configFile)
				// fmt.Println(qmConfig)
				qmConfig.DiffConfig(qmConfig.ResourcesWatch, newConfig.ResourcesWatch)

				//stop watches if we have any to stop
				stopWatchers(doneChans, qmConfig)

				// start any new watchers
				startWatchers(clientset, qmConfig)

			}
		}
	}()

	// create channel to watch for SIGNALs to exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting.")
}
