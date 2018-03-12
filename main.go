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

	// loop through list of resources to watch and startup watchers
	// for those resources.
	for _, resource := range qmConfig.ResourceWatch {
		switch resource {
		case "namespaces":
			// watch namespaces
			go func() {
				cluster.Namespaces(clientset, qmConfig)
			}()
		case "pods":
			// watch pods
			go func() {
				cluster.Pods(clientset, qmConfig)
			}()
		case "deployments":
			// watch deployments
			go func() {
				cluster.Deployments(clientset, qmConfig)
			}()
		}
	}

	// create channel to watch for SIGNALs to exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting.")
}
