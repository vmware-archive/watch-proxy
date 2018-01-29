package cluster

import (
	"log"

	"github.com/heptio/clerk/inventory"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Version - get the version of k8s running on the cluster
func Version(client *kubernetes.Clientset) string {

	version, err := client.DiscoveryClient.ServerVersion()
	if err != nil {
		log.Println("Could not get server version", err)
	}
	return version.GitVersion

}

// Namespaces - Get list of namespaces running on the cluster
func Namespaces(client *kubernetes.Clientset) []string {

	nsData, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		log.Println("Could not get list of namespaces", err)
	}

	namespaces := []string{}
	for _, ns := range nsData.Items {
		namespaces = append(namespaces, ns.ObjectMeta.Name)
	}

	return namespaces
}

// Deployments - get list of deployments running in a specified namespace
// and returns an array of inventory Deployment types
func Deployments(client *kubernetes.Clientset, ns string) []inventory.Deployment {
	deployments, err := client.AppsV1beta2().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		log.Println("Could not get list of deployments", err)
	}

	// create empty array of deployments to return
	minDeployments := []inventory.Deployment{}

	for _, deployment := range deployments.Items {
		minDeployment := inventory.Deployment{
			Name:            deployment.ObjectMeta.Name,
			Namespace:       ns,
			Labels:          deployment.ObjectMeta.Labels,
			ReplicasDesired: *deployment.Spec.Replicas,
		}
		minDeployments = append(minDeployments, minDeployment)

	}

	return minDeployments
}
