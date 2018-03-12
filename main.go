package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/heptio/quartermaster/cluster"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var remoteEnd *string

func main() {

	remoteEnd = flag.String("remote", "", "URL to remote end point that will receive updates")
	flag.Parse()

	if *remoteEnd == "" {
		// if we didn't get a flag pased to use check env vars
		*remoteEnd = os.Getenv("REMOTE_ENDPOINT")
	}
	if *remoteEnd == "" {
		// and if remoteEnd is still empty throw and error
		log.Fatalln("Remote Endpoint URL is required")
	}

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
		cluster.Namespaces(clientset, *remoteEnd)
	}()

	// watch deployments
	go func() {
		cluster.Deployments(clientset, *remoteEnd)
	}()

	// watch pods
	go func() {
		cluster.Pods(clientset, *remoteEnd)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting.")
}
