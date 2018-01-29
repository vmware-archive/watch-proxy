package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpweber/kinv/inventory"

	"github.com/jpweber/kinv/cluster"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func main() {
	var kubeconfig *string

	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// get the cluster version
	version := cluster.Version(clientset)

	// Init our cluster inventory with the version
	inv := inventory.Cluster{
		Version: version,
	}

	// get the namespaces
	inv.Namespaces = cluster.Namespaces(clientset)

	// init nsDeployments var for use in loop
	nsDeployments := make(map[string][]inventory.Deployment, len(inv.Namespaces))
	// fetch deployment information
	// nsDeployments := make(map[string][]inventory.Deployment)
	for _, ns := range inv.Namespaces {
		nsDeployments[ns] = cluster.Deployments(clientset, ns)
	}

	// take our adhoc nsDeployments and add it to the inventory struct
	inv.Deployments = nsDeployments

	// testing
	fmt.Printf("%+v", inv.Deployments)
}
