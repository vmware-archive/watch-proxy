// Copyright 2018-2019 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package kubecluster

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	networking_v1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	lister_apps_v1 "k8s.io/client-go/listers/apps/v1"
	lister_core_v1 "k8s.io/client-go/listers/core/v1"
	lister_networking_v1beta1 "k8s.io/client-go/listers/networking/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	addKey    = "add"
	updateKey = "update"
	DeleteKey = "delete"
)

// Listers which hold cached k8s objects for later lookup
var (
	NsLister     lister_core_v1.NamespaceLister
	PoLister     lister_core_v1.PodLister
	NoLister     lister_core_v1.NodeLister
	CmLister     lister_core_v1.ConfigMapLister
	SvcLister    lister_core_v1.ServiceLister
	SecretLister lister_core_v1.SecretLister
	DeployLister lister_apps_v1.DeploymentLister
	RsLister     lister_apps_v1.ReplicaSetLister
	IngLister    lister_networking_v1beta1.IngressLister
)

// Informer is capable of starting and stopping an running informer.
type Informer interface {
	Start()
	Stop()
}

// InformerClient contains configuration for instantiating an informer. Use NewInformerClient to
// ensure valid configuration is provided.
type InformerClient struct {
	client           *kubernetes.Clientset
	rest             *rest.Interface
	resource         string
	ignoreNamespaces []string
	k8sObjectType    runtime.Object
	resyncTime       time.Duration
	processQueue     workqueue.RateLimitingInterface
	done             chan bool
	allowAddEvent    bool
	skipAddEventTime time.Duration
	clusterName      string
}

// InformerClients is a wrapper around []*InformerClient to allow methods to be added such as the
// need to find an InformerClient within the list based on resource.
type InformerClients []*InformerClient

// NewInformerClient returns an InformerClient capable of starting an informer to watch all events
// a specific k8s object type. It returns an error when the request object type (specified in the
// resource argument is not known to quartermaster. The arguments it takes are as follows.
//
// * client: kubernetes.Clientset used for generating REST clients capable for communicating with
//           kubernetes
// * resource: the type of k8s object the informer will watch for events on. e.g. pods,
//             deployments, or namespaces
// * nsSelector: scopes the informer to only watch objects in a specific namespace. An empty string
//               represents all namespaces.
// * pQueue: processor queue where all events should be dropped for future processing.
func NewInformerClient(client *kubernetes.Clientset, resource string, nsSelector string,
	pQueue workqueue.RateLimitingInterface, config config.Config) (*InformerClient, error) {
	r, obj, err := getRuntimeObjectConfig(client, resource)

	if err != nil {
		return nil, err
	}

	delay, err := time.ParseDuration(config.DelayStartSeconds)
	if err != nil {
		delay = 0 * time.Second
		glog.Warningf("%s: no valid delayAddEventDuration, quartermaster will process all events without delay. error: %s. "+
			"no delay will be applied.", resource, err.Error())
	}

	resyncDuration, err := time.ParseDuration(config.ForceReuploadDuration)
	if err != nil {
		resyncDuration = 0 * time.Second
		glog.Warningf("%s: no valid forceReuploadDuration set, quartermaster will not attempt to periodically re-upload"+
			" all kubernetes objects.", resource)
	}

	doneChannel := make(chan bool)
	ic := &InformerClient{
		rest:             r,
		ignoreNamespaces: config.IgnoreNamespaces,
		k8sObjectType:    obj,
		resource:         resource,
		resyncTime:       resyncDuration,
		processQueue:     pQueue,
		done:             doneChannel,
		skipAddEventTime: delay,
		clusterName:      config.ClusterName,
	}
	return ic, nil
}

// configureUID takes a kubernetes object and traverses the metadata field to locate the value of the UID
// if not UID is found or the metadata is non-existent, an empty string is returned. Additionally i adds the field
// metadata.uniqueID to the object to be emitted.
func configureUID(obj interface{}) string {
	// TODO(joshrosso): There has to be a better way to do this than marshling in and out of JSON
	b, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	var k8sObj map[string]interface{}
	err = json.Unmarshal(b, &k8sObj)
	if err != nil {
		return ""
	}

	metadata, ok := k8sObj["metadata"]
	if !ok {
		glog.Errorf("Failed to locate metadata.uid used for emitting object.")
		return ""
	}

	return fmt.Sprintf("%s", metadata.(map[string]interface{})["uid"])
}

// Start instantiates an informer and begins the watch for resource events. The informer's
// resulting controller is run in its own go routine and Start will block until a signal is sent to
// the InformerClient's Done channel. Upon that signal, the controller's go routine will is stopped
// Start will return.
func (ic InformerClient) Start() {
	if ic.skipAddEventTime > 0*time.Second {
		go ic.startSkipAddEventTimer()
	} else {
		ic.allowAddEvent = true
	}

	// watcher and lister configuration for informer
	watchlist := cache.NewListWatchFromClient(*ic.rest, ic.resource, "", fields.Everything())

	// eventhandlers describing what to do upon add, update, and delete events.
	eHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !ic.ignoreNamespace(obj) {
				if !ic.addEventAllowed() {
					glog.Infof("skipping add for %s. start delay in effect", configureUID(obj))
					return
				}
				if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
					ic.processQueue.AddRateLimited(fmt.Sprintf("%s|%s|%s|%s|x", addKey, ic.clusterName+"-"+configureUID(obj),
						ic.resource, key))
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if !ic.ignoreNamespace(newObj) {
				if key, err := cache.MetaNamespaceKeyFunc(newObj); err == nil {
					ic.processQueue.AddRateLimited(fmt.Sprintf("%s|%s|%s|%s|x", updateKey, ic.clusterName+"-"+configureUID(newObj),
						ic.resource, key))
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			if !ic.ignoreNamespace(obj) {
				if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
					mObj, err := json.Marshal(obj)
					if err != nil {
						glog.Errorf("failed to marshal deleted object: %s", err)
					}
					ic.processQueue.AddRateLimited(fmt.Sprintf("%s|%s|%s|%s|%s", DeleteKey, ic.clusterName+"-"+configureUID(obj),
						ic.resource, key, mObj))
				}
			}
		},
	}

	// informer creation, returning indexer used in lister generation and controller capable of
	// starting event watches.
	indexer, controller := cache.NewIndexerInformer(watchlist, ic.k8sObjectType, ic.resyncTime,
		eHandlers, cache.Indexers{})

	// attempt to create globally accessible listers that can be used to lookup k8s objects that
	// events had previously fired for.
	err := initLister(indexer, ic.k8sObjectType)
	if err != nil {
		glog.Errorln(err.Error())
		return
	}

	// run the controller and block until a stop signal is received via the ic.Done channel. upon
	// receiving the signal, stop the controller and return from this function.
	stop := make(chan struct{})
	go controller.Run(stop)
	glog.Infof("informer is active for resource: %s", ic.resource)
	<-ic.done
	close(stop)
	glog.Infof("informer has been stopped for resource: %s", ic.resource)
}

// Stop tells the InformerClient's controller (watcher, indexer, etc) to stop.
func (ic InformerClient) Stop() {
	ic.done <- true
}

// FindInformerClient looks up an InformerClient responsible for handling the passes resource.
func (ics InformerClients) FindInformerClient(resource string) *InformerClient {
	for _, ic := range ics {
		if ic.resource == resource {
			return ic
		}
	}
	glog.Errorf("Expected to find existing informer client for resource %s. Found nothing.",
		resource)
	return nil
}

// RemoveInformerClient returns a new InformerClients slice with the passed removeIc
// (InformerClient) removed from the list.
func RemoveInformerClient(ics InformerClients, removeIc *InformerClient) InformerClients {
	for i, ic := range ics {
		if ic == removeIc {
			ics = append(ics[:i], ics[i+1:]...)
		}
	}
	return ics
}

// getRuntimeObjectConfig returns the appropriate rest client and runtime object type based on the
// resource argument. the kubernetes.Clientset argument is used to construct the rest client.
func getRuntimeObjectConfig(client *kubernetes.Clientset, resource string) (*rest.Interface, runtime.Object, error) {

	var rest rest.Interface
	var obj runtime.Object

	switch resource {
	case "namespaces":
		rest = client.CoreV1().RESTClient()
		obj = &core_v1.Namespace{}
	case "pods":
		rest = client.CoreV1().RESTClient()
		obj = &core_v1.Pod{}
	case "nodes":
		rest = client.CoreV1().RESTClient()
		obj = &core_v1.Node{}
	case "configmaps":
		rest = client.CoreV1().RESTClient()
		obj = &core_v1.ConfigMap{}
	case "secrets":
		rest = client.CoreV1().RESTClient()
		obj = &core_v1.Secret{}
	case "services":
		rest = client.CoreV1().RESTClient()
		obj = &core_v1.Service{}
	case "ingresses":
		rest = client.NetworkingV1beta1().RESTClient()
		obj = &networking_v1beta1.Ingress{}
	case "deployments":
		rest = client.AppsV1().RESTClient()
		obj = &apps_v1.Deployment{}
	case "replicasets":
		rest = client.AppsV1().RESTClient()
		obj = &apps_v1.ReplicaSet{}
	default:
		return nil, nil, fmt.Errorf("object type requested is not recognized. type: %s", resource)
	}

	return &rest, obj, nil
}

// initLister initializes a globally accessible k8s object lister based objType passed. objType
// should be one of runtime.Object. The indexer argument must be the indexer that's returned upon
// generating an informer; a step that is done when calling Informer.Start(). This globally
// accessible lister is used by the processor package to lookup, via cache, objects that may
// eventually be emitted.
func initLister(i cache.Indexer, objType interface{}) error {
	switch t := objType.(type) {
	case *core_v1.Namespace:
		NsLister = lister_core_v1.NewNamespaceLister(i)
	case *core_v1.Pod:
		PoLister = lister_core_v1.NewPodLister(i)
	case *core_v1.Node:
		NoLister = lister_core_v1.NewNodeLister(i)
	case *core_v1.ConfigMap:
		CmLister = lister_core_v1.NewConfigMapLister(i)
	case *core_v1.Service:
		SvcLister = lister_core_v1.NewServiceLister(i)
	case *networking_v1beta1.Ingress:
		IngLister = lister_networking_v1beta1.NewIngressLister(i)
	case *core_v1.Secret:
		SecretLister = lister_core_v1.NewSecretLister(i)
	case *apps_v1.Deployment:
		DeployLister = lister_apps_v1.NewDeploymentLister(i)
	case *apps_v1.ReplicaSet:
		RsLister = lister_apps_v1.NewReplicaSetLister(i)
	default:
		return fmt.Errorf("Failed to init lister due to inability to infer type. Type was %s", t)
	}
	return nil
}

// addEventAllowed checks for whether the add event should be skipped. This is based on a timer that is
// set when a skipAddEventTime is set.
func (ic InformerClient) addEventAllowed() bool {
	return ic.allowAddEvent
}

// ingoreNamespace checks to see if the object's namespace is being ignored
// if an object's namespace is in the ignore list, true is returned
// if an object's namespace is *not* in the ignore list, false is returned
func (ic InformerClient) ignoreNamespace(obj interface{}) bool {

	b, err := json.Marshal(obj)
	if err != nil {
		glog.Errorf("Failed to marshal object json: %s", err)
	}

	var k8sObj map[string]interface{}
	err = json.Unmarshal(b, &k8sObj)
	if err != nil {
		glog.Errorf("Failed to unmarshal K8s object json: %s", err)
	}

	metadata, ok := k8sObj["metadata"]
	if !ok {
		glog.Errorf("Failed to identify namespace for %s", metadata.(map[string]interface{})["selfLink"].(string))
	}

	for _, n := range ic.ignoreNamespaces {
		selfLink := metadata.(map[string]interface{})["selfLink"].(string)
		if strings.Contains(selfLink, fmt.Sprintf("namespaces/%s", n)) == true {
			return true
		}
	}

	return false
}

// startSkipAddEventTimer creates a timer for the duration set in the InformerClient's
// skipAddEventTime attribute. It blocks until the timer has finished then sets the allowedAddEvent
// flag to true, signifying that the client can now queue add events it receives from the
// kubernetes API server.
func (ic *InformerClient) startSkipAddEventTimer() {
	t := time.NewTimer(ic.skipAddEventTime)
	// wait for return on timer's channel then set allowAddEvent flag to true
	glog.Infof("add event delay in effect for %s resource watch for %v duration", ic.resource,
		ic.skipAddEventTime)
	<-t.C
	ic.allowAddEvent = true
	glog.Infof("add event delay ended for %s resource watch", ic.resource)
}

// String pretty prints InformerClients.
func (ics InformerClients) String() string {
	o := "["
	for i, ic := range ics {
		o += ic.resource
		if i != len(ics)-1 {
			o += ", "
		}
	}
	o += "]"
	return o
}
