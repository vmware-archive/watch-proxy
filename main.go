package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	"github.com/heptio/quartermaster/emitter"
	"github.com/heptio/quartermaster/kubecluster"
	"github.com/heptio/quartermaster/processor"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

var (
	kubeconfigPath string
	configFile     string
)

func main() {
	parseFlags()
	glog.Infoln("starting quartermaster")

	// create kubernetes client
	clientset, err := kubecluster.NewK8sClient(kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	// create quartermaster configuration
	parsedConfig, err := config.ReadConfig(configFile)
	if err != nil {
		panic(err.Error())
	}
	qmConfig := *parsedConfig

	// create workqueue where all objects triggered by events go and start processor that reads
	// from the queue
	processor.Queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	processor.StartProcessor(qmConfig.ResourcesWatch)

	// start emitter for sending object to remote endpoint
	emitter.StartEmitter(qmConfig, make(chan emitter.EmitObject, 1000))

	// start all k8s object watchers
	ics := kubecluster.StartWatchers(clientset, qmConfig, processor.Queue)

	// watch for changes to the config file and and adjust watchers if there are changes
	fileChange := make(chan bool)
	config.NewFileWatcher(fileChange, configFile)

	// start the loop that reloads the configuration and starts or stops
	watchConfiguration(ics, fileChange, qmConfig, clientset)

	// create channel to watch for SIGNALs to exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	glog.Infoln("Shutdown signal received, exiting.")
}

func parseFlags() {
	// When running as a pod in-cluster, a kubeconfig is not needed. Instead this will make use of
	// the service account injected into the pod. However, allow the use of a local kubeconfig as
	// this can make local development & testing easier.
	kubeconfigPath = *flag.String("kubeconfig", "",
		"Path to a kubeconfig file")
	configFile = *flag.String("c",
		"/etc/quartermaster/config.yaml",
		"Path to quartermaster config file")
	// We log to stderr because glog will default to logging to a file.
	// By setting this debugging is easier via `kubectl logs`
	flag.Set("logtostderr", "true")
	flag.Parse()
}

func watchConfiguration(ics kubecluster.InformerClients, fileChange chan bool,
	qmConfig config.Config, clientset *kubernetes.Clientset) {
	for {
		<-fileChange
		glog.Infof("config file change detected")

		// create new config from updated config file
		newConfig, err := config.ReadConfig(configFile)
		// if the new config is invalid, skip it until next update
		if err != nil {
			glog.Errorf("configuration file was invalid with error: %s.", err.Error())
			break
		}

		// compare the config structs so we know what watchers to stop and start
		qmConfig.DiffConfig(qmConfig.ResourcesWatch, newConfig.ResourcesWatch)

		// stop watchers no longer desired
		if len(qmConfig.StaleResources) > 0 {
			stoppedIcs := kubecluster.StopWatchers(ics, qmConfig)
			glog.Infof("stopped ICS: %s", stoppedIcs)
			for _, stoppedIc := range stoppedIcs {
				ics = kubecluster.RemoveInformerClient(ics, stoppedIc)
			}
			glog.Infof("stop completed. current running informers are: %s", ics)
		}

		// start new watchers
		if len(qmConfig.NewResources) > 0 {
			addedIcs := kubecluster.StartWatchers(clientset, qmConfig, processor.Queue)
			processor.SetPruneFields(qmConfig.NewResources)
			emitter.SetAssetIds(qmConfig.NewResources)
			for _, addedIc := range addedIcs {
				ics = append(ics, addedIc)
			}
		}
		glog.Infof("finished reloading config. watchers exist for the following resource type"+
			"s: %s", ics)
	}

}
