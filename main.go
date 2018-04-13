package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/heptio/quartermaster/config"
	"github.com/heptio/quartermaster/kubecluster"
)

var remoteEnd *string

func main() {
	// When running as a pod in-cluster, a kubeconfig is not needed. Instead this will make use of the service account injected into the pod.
	// However, allow the use of a local kubeconfig as this can make local development & testing easier.
	kubeconfigPath := *flag.String("kubeconfig", "", "Path to a kubeconfig file")
	configFile := *flag.String("c", "/etc/quartermaster/config.yaml", "Path to quartermaster config file")
	flag.Parse()

	parsedConfig, err := config.ReadConfig(configFile)
	if err != nil {
		panic(err.Error())
	}
	qmConfig := *parsedConfig

	//TOOD(joshrosso): add flag parsing for getting literal kubeconfig
	clientset, err := kubecluster.NewK8sClient(kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	// only init a whole cluster object if we are NOT doing delta updates
	// TODO(joshrosso): get clarity on why we need this if, appears this exact call will be made below?
	if !qmConfig.DeltaUpdates {
		kubecluster.Initialize(clientset, qmConfig)
	}

	kubecluster.Initialize(clientset, qmConfig)

	// fire up the watchers
	doneChans := kubecluster.StartWatchers(clientset, qmConfig)

	// watch for changes to the config file and
	// reload if the config and adjust watchers if there are changes
	fileChange := make(chan bool)

	// create new file watcher on specified file
	config.NewFileWatcher(fileChange, configFile)

	// Start the loop that reloads the configuration and starts or stops
	go func() {
		for {
			select {
			case _ = <-fileChange:
				log.Println("Updating Config")
				// create new config from updated config file
				newConfig, err := config.ReadConfig(configFile)
				// if the new config is invalid, skip it until next update
				if err != nil {
					log.Printf("configuration file was invalid with error: %s.", err.Error())
					break
				}

				// compare the config structs so we know what watchers to stop and start
				qmConfig.DiffConfig(qmConfig.ResourcesWatch, newConfig.ResourcesWatch)

				//stop watches if we have any to stop
				if len(qmConfig.StaleResources) > 0 {
					kubecluster.StopWatchers(doneChans, qmConfig)
				}

				// start any new watchers
				if len(qmConfig.NewResources) > 0 {
					kubecluster.StartWatchers(clientset, qmConfig)
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
