package cluster

import (
	"log"
	"time"

	"github.com/heptio/clerk/emitter"

	"github.com/heptio/clerk/inventory"

	"k8s.io/api/apps/v1beta2"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Version - get the version of k8s running on the cluster
func Version(client *kubernetes.Clientset) string {

	version, err := client.DiscoveryClient.ServerVersion()
	if err != nil {
		log.Println("Could not get server version", err)
	}
	return version.GitVersion

}

// Namespaces - Emit events on Namespace create and deletions
func Namespaces(client *kubernetes.Clientset) {

	watchlist := cache.NewListWatchFromClient(client.Core().RESTClient(), "namespaces", v1.NamespaceAll,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Namespace{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ns := obj.(*v1.Namespace)
				inv := inventory.Namespace{
					Name:  ns.ObjectMeta.Name,
					Event: "created",
					Kind:  "namespace",
				}

				// sent the update to remote endpoint
				emitter.EmitChanges(inv)

				log.Printf("Namespace Created: %s",
					ns.ObjectMeta.Name,
				)

			},
			DeleteFunc: func(obj interface{}) {
				ns := obj.(*v1.Namespace)

				inv := inventory.Namespace{
					Name:  ns.ObjectMeta.Name,
					Event: "deleted",
					Kind:  "namespace",
				}

				// sent the update to remote endpoint
				emitter.EmitChanges(inv)

				log.Printf("Namespace Deleted: %s",
					ns.ObjectMeta.Name,
				)

			},
		},
	)

	stop := make(chan struct{})
	done := make(chan bool)
	go controller.Run(stop)
	log.Println("Started Watching Namespaces")
	<-done

}

// Deployments - Emit events on Deployment changes
func Deployments(client *kubernetes.Clientset) {
	watchlist := cache.NewListWatchFromClient(client.AppsV1beta2().RESTClient(), "deployments", v1.NamespaceAll,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1beta2.Deployment{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dep := obj.(*v1beta2.Deployment)
				log.Printf("Deployment Created: %s In NameSpace: %s",
					dep.ObjectMeta.Name,
					dep.ObjectMeta.Namespace,
					// TODO: this don't log correctly
					// but we are only logging for dev purposed currently
					dep.ObjectMeta.Labels,
				)

			},
			DeleteFunc: func(obj interface{}) {
				dep := obj.(*v1beta2.Deployment)
				log.Printf("Deployment Deleted: %s, In NameSpace: %s",
					dep.ObjectMeta.Name,
					dep.ObjectMeta.Namespace,
					// TODO: this don't log correctly
					// but we are only logging for dev purposed currently
					dep.ObjectMeta.Labels,
				)

			},
		},
	)

	stop := make(chan struct{})
	done := make(chan bool)
	go controller.Run(stop)
	log.Println("Started Watching Deployments")
	<-done
}
