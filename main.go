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
	doneChans := cluster.StartWatchers(clientset, qmConfig)

	// watch for changes to the config file and
	// reload if the config and adjust watchers if there are changes
	fileChange := make(chan bool)

	// create new file watcher on specified file
	config.NewFileWatcher(fileChange, *configFile)

	// Start the loop that reloads the configuration and starts or stops
	go func() {
		for {
			select {
			case _ = <-fileChange:
				log.Println("Updating Config")
				// create new config from updated config file
				newConfig := config.ReadConfig(*configFile)
				// compare the config structs so we know what watchers to stop and start
				qmConfig.DiffConfig(qmConfig.ResourcesWatch, newConfig.ResourcesWatch)

				//stop watches if we have any to stop
				if len(qmConfig.StaleResources) > 0 {
					cluster.StopWatchers(doneChans, qmConfig)
				}

				// start any new watchers
				if len(qmConfig.NewResources) > 0 {
					cluster.StartWatchers(clientset, qmConfig)
				}

			}
		}
	}()

	// create channel to watch for SIGNALs to exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting.")
}
