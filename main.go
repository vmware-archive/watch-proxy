package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/heptio/clerk/cluster"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// get the cluster version
	// version := cluster.Version(clientset)

	// watch namespaces
	go func() {
		cluster.Namespaces(clientset)
	}()

	// watch deployments
	go func() {
		cluster.Deployments(clientset)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting.")
}
