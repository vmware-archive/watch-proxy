package kubecluster

import (
	"log"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"k8s.io/apimachinery/pkg/types"

	"github.com/heptio/quartermaster/config"
	"github.com/heptio/quartermaster/emitter"

	"github.com/heptio/quartermaster/inventory"

	"k8s.io/api/apps/v1beta2"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// StartWatchers - start up the k8s resource watchers
func StartWatchers(client *kubernetes.Clientset, config config.Config) map[string]chan bool {
	// loop through list of resources to watch and startup watchers
	// for those resources.
	resources := []string{}
	// decide if we want to load the full set of resources. Usually at startup
	// or just start up new watchers after config change
	if len(config.NewResources) > 0 {
		resources = config.NewResources
	} else {
		resources = config.ResourcesWatch
	}

	// create map to capture the channels used to signal done to the watcher
	// go routines
	doneChannels := make(map[string]chan bool)

	// start the watchers
	for _, resource := range resources {
		switch resource {
		case "namespaces":
			nameDone := make(chan bool)
			doneChannels[resource] = nameDone
			// watch namespaces
			go func() {
				Namespaces(client, config, nameDone)
			}()
		case "pods":
			// watch pods
			podDone := make(chan bool)
			doneChannels[resource] = podDone
			// watch pods
			go func() {
				Pods(client, config, podDone)
			}()
		case "deployments":
			depDone := make(chan bool)
			doneChannels[resource] = depDone
			// watch deployments
			go func() {
				Deployments(client, config, depDone)
			}()
		}
	}

	// return the map of done channels so we can stop things later if need be
	return doneChannels
}

// StopWatchers - stop the watchers we no longer want running
func StopWatchers(doneChannels map[string]chan bool, config config.Config) {
	// loop through list of stale resource types and stop those watchers
	for _, staleRes := range config.StaleResources {
		log.Println("Stopping", staleRes)
		// send true to the done channel for the respective resource
		// to close the go routine
		doneChannels[staleRes] <- true
	}
}

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
	close(stop)
	log.Println("Stopped Watching Namespaces")

}

// Deployments - Emit events on Deployment changes
func Deployments(client *kubernetes.Clientset, config config.Config, done chan bool) {
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
					UID:             NewUID(),
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
					UID:             NewUID(),
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	log.Println("Started Watching Deployments")
	<-done
	// if we have made it past done close the stop channel
	// to tell the controller to stop as well.
	close(stop)
	log.Println("Stopped Watching Deployments")
}

func Pods(client *kubernetes.Clientset, config config.Config, done chan bool) {

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
					UID:       NewUID(),
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
					UID:       NewUID(),
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
					UID:       NewUID(),
				}
				// sent the update to remote endpoint
				emitter.EmitChanges(inv, config.RemoteEndpoint)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	log.Println("Started Watching Pods")
	<-done
	// if we have made it past done close the stop channel
	// to tell the controller to stop as well.
	close(stop)
	log.Println("Stopped Watching Pods")
}

func imagesFromContainers(containers []v1.Container) []string {

	images := []string{}
	for _, cont := range containers {
		images = append(images, cont.Image)
	}

	return images
}

var uuidLock sync.Mutex
var lastUUID uuid.UUID

// Copied from the Kubernetes repo: https://github.com/kubernetes/apimachinery/blob/release-1.10/pkg/util/uuid/uuid.go
func NewUID() types.UID {
	uuidLock.Lock()
	defer uuidLock.Unlock()
	result := uuid.NewUUID()
	// The UUID package is naive and can generate identical UUIDs if the
	// time interval is quick enough.
	// The UUID uses 100 ns increments so it's short enough to actively
	// wait for a new value.
	for uuid.Equal(lastUUID, result) == true {
		result = uuid.NewUUID()
	}
	lastUUID = result
	return types.UID(result.String())
}
