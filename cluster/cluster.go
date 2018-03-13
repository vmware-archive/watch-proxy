package cluster

import (
	"log"
	"time"

	"github.com/heptio/quartermaster/config"
	"github.com/heptio/quartermaster/emitter"

	"github.com/heptio/quartermaster/inventory"

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
func Namespaces(client *kubernetes.Clientset, config config.Config, done chan bool) {

	watchlist := cache.NewListWatchFromClient(client.Core().RESTClient(), "namespaces", v1.NamespaceAll,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Namespace{},
		time.Second*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ns := obj.(*v1.Namespace)
				inv := inventory.Namespace{
					Name:  ns.ObjectMeta.Name,
					Event: "created",
					Kind:  "namespace",
				}

				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)

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
				emitter.EmitChanges(inv, config.RemoteEndpoint)

				log.Printf("Namespace Deleted: %s",
					ns.ObjectMeta.Name,
				)

			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	log.Println("Started Watching Namespaces")
	<-done
	// if we have made it past done close the stop channel
	// to tell the controller to stop as well.
	log.Println("Stopped Watching Namespaces")
	close(stop)

}

// Deployments - Emit events on Deployment changes
func Deployments(client *kubernetes.Clientset, config config.Config) {
	watchlist := cache.NewListWatchFromClient(client.AppsV1beta2().RESTClient(), "deployments", v1.NamespaceAll,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1beta2.Deployment{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dep := obj.(*v1beta2.Deployment)
				inv := inventory.Deployment{
					Name:            dep.ObjectMeta.Name,
					Namespace:       dep.ObjectMeta.Namespace,
					Labels:          dep.ObjectMeta.Labels,
					ReplicasDesired: *dep.Spec.Replicas,
					Event:           "created",
					Kind:            "deployment",
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},
			DeleteFunc: func(obj interface{}) {
				dep := obj.(*v1beta2.Deployment)
				inv := inventory.Deployment{
					Name:            dep.ObjectMeta.Name,
					Namespace:       dep.ObjectMeta.Namespace,
					Labels:          dep.ObjectMeta.Labels,
					ReplicasDesired: *dep.Spec.Replicas,
					Event:           "deleted",
					Kind:            "deployment",
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},
		},
	)

	stop := make(chan struct{})
	done := make(chan bool)
	go controller.Run(stop)
	log.Println("Started Watching Deployments")
	<-done
}

func Pods(client *kubernetes.Clientset, config config.Config) {

	watchlist := cache.NewListWatchFromClient(client.Core().RESTClient(), "pods", v1.NamespaceAll,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				inv := inventory.Pod{
					Name:      pod.ObjectMeta.Name,
					Namespace: pod.ObjectMeta.Namespace,
					Labels:    pod.ObjectMeta.Labels,
					Images:    imagesFromContainers(pod.Spec.Containers),
					Event:     "created",
					Kind:      "pod",
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},

			DeleteFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				inv := inventory.Pod{
					Name:      pod.ObjectMeta.Name,
					Namespace: pod.ObjectMeta.Namespace,
					Labels:    pod.ObjectMeta.Labels,
					Images:    imagesFromContainers(pod.Spec.Containers),
					Event:     "deleted",
					Kind:      "pod",
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},

			UpdateFunc: func(_, obj interface{}) {
				pod := obj.(*v1.Pod)
				inv := inventory.Pod{
					Name:      pod.ObjectMeta.Name,
					Namespace: pod.ObjectMeta.Namespace,
					Labels:    pod.ObjectMeta.Labels,
					Images:    imagesFromContainers(pod.Spec.Containers),
					Event:     "modified",
					Kind:      "pod",
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},
		},
	)
	stop := make(chan struct{})
	done := make(chan bool)
	go controller.Run(stop)
	log.Println("Started Watching Pods")
	<-done
}

func imagesFromContainers(containers []v1.Container) []string {

	images := []string{}
	for _, cont := range containers {
		images = append(images, cont.Image)
	}

	return images
}
