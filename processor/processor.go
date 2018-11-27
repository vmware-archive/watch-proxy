// Package processor implements the logic for reading queued events that come from configured
// informers.
package processor

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	"github.com/heptio/quartermaster/emitter"
	"github.com/heptio/quartermaster/kubecluster"
	"k8s.io/client-go/util/workqueue"
)

const processWaitTime = 100 * time.Millisecond

var (
	Queue           workqueue.RateLimitingInterface
	PruneFields     map[string][]string
	PruneFieldsLock sync.RWMutex
)

// StartProcessor starts the go routine responsible for checking to see if there are events from
// informers that should make there way to the emit queue.
func StartProcessor(resources []config.Resource) {
	PruneFieldsLock = sync.RWMutex{}
	SetPruneFields(resources)
	glog.Infoln("started processor for reading events off queue")
	go runProcessor()
}

func runProcessor() {
	for processNext() {
		time.Sleep(processWaitTime)
	}
}

// processNext checks the queue where informers drop objects based on deleted, added, and updated
// events.
func processNext() bool {
	// queue might still be initializing
	if Queue == nil {
		glog.Warningf("informer work queue still initializing; waiting.")
		return true
	}
	// Wait until there is a new item in the working queue
	key, quit := Queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer Queue.Done(key)
	// Invoke the method containing the business logic
	err := process(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	handleErr(err, key)
	return true
}

func handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		Queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if Queue.NumRequeues(key) < 5 {
		glog.Errorf("Error processing object %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		Queue.AddRateLimited(key)
		return
	}

	Queue.Forget(key)
	glog.Errorf("Dropping object %q out of the queue: %v", key, err)
}

func process(key string) error {
	obj, err := getK8sObject(key)
	if err != nil {
		glog.Errorf(err.Error())
		return err
	}

	// check whether object was previously emitted in exact state; if so, do nothing.
	if emitter.WasEmitted(*obj) {
		glog.Infof("[%s]: not emitting, state has not changed since last event",
			obj.Key)
		return nil
	}
	glog.Infof("[%s]: queued to emit", obj.Key)
	emitter.EmitQueue <- *obj

	return nil
}

// getK8sObject looks up the actual kubernetes object via the lister (cache). It selects the
// appropriate lister based inferring the object type from the key. A "|" character is expected to
// delimit the type from the lookup key itself.
func getK8sObject(key string) (*emitter.EmitObject, error) {
	objType := strings.Split(key, "|")
	var obj interface{}
	if len(objType) < 4 {
		return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
	}
	operation := objType[0]
	uid := objType[1]

	// when operation is delete, don't worry about looking up the objects data, it likely doesn't
	// exist in the lister/indexer anyways.
	if operation == kubecluster.DeleteKey {
		splitType := []string{objType[2], objType[3]}
		return &emitter.EmitObject{nil, splitType[0], objType[0] + "|" + objType[1], uid,
			operation}, nil
	}

	// update or add operation occured, lookup object in lister and create emittable object
	objType = []string{objType[2], objType[3]}
	glog.Infof("[%s]: event triggered", key)
	switch objType[0] {
	case "namespaces":
		obj, _ = kubecluster.NsLister.Get(objType[1])

	case "nodes":
		obj, _ = kubecluster.NoLister.Get(objType[1])

	case "pods":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.PoLister.Pods(lookupKey[0]).Get(lookupKey[1])

	case "services":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.SvcLister.Services(lookupKey[0]).Get(lookupKey[1])

	case "ingresses":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.IngLister.Ingresses(lookupKey[0]).Get(lookupKey[1])

	case "deployments":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.DeployLister.Deployments(lookupKey[0]).Get(lookupKey[1])

	case "replicasets":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.RsLister.ReplicaSets(lookupKey[0]).Get(lookupKey[1])

	case "configmaps":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.CmLister.ConfigMaps(lookupKey[0]).Get(lookupKey[1])

	case "secrets":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.SecretLister.Secrets(lookupKey[0]).Get(lookupKey[1])

	case "virtualservices":
		lookupKey := strings.Split(objType[1], "/")
		if len(lookupKey) < 2 {
			return nil, fmt.Errorf("k8s object key was invalid, couldn't lookup in lister.")
		}
		obj, _ = kubecluster.VsLister.VirtualServices(lookupKey[0]).Get(lookupKey[1])

	default:
		return nil, fmt.Errorf("k8s object type is unknown. type was: %s", objType[0])

	}

	jsonBody, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	mapO := make(map[string]interface{})
	json.Unmarshal(jsonBody, &mapO)
	fieldsToPrune := lookupPruneFields(objType[0])
	for _, fieldToPrune := range fieldsToPrune {
		fieldToPrune := strings.Split(fieldToPrune, ".")
		prune(fieldToPrune, mapO)
	}

	// inside the object's metadata field, add the uniqueID
	if metadata, ok := mapO["metadata"]; ok {
		metadata.(map[string]interface{})["uniqueId"] = uid
	}

	return &emitter.EmitObject{mapO, objType[0], objType[0] + "|" + objType[1], uid, operation},
		nil
}

// prune receives a list of fields to use for recursive traversal towards a desired key you'd
// like to remove. For example, if you'd like to remove the json field metadata.creationTimestamp,
// the passed pruneFields should be []string{"metadata", "creationTimestamp"}. The obj parameters
// should be a map[string]{interface} representation of JSON data. You can use json.Unmarshal to
// achieve this. If recursive traversal fails to find the specified key, no pruning occurs.
func prune(pruneFields []string, obj map[string]interface{}) {
	if len(pruneFields) < 1 {
		return
	}

	// prune the final field and end rescursion
	if len(pruneFields) == 1 {
		delete(obj, pruneFields[0])
		return
	}

	nMap, ok := obj[pruneFields[0]]
	// if key didn't exist, end traversal and return
	if !ok {
		return
	}
	prune(append(pruneFields[:0], pruneFields[0+1:]...),
		nMap.(map[string]interface{}))
}

// lookupPruneFields finds the returns a list of the fields that should be pruned for the requested
// resource.
func lookupPruneFields(resource string) []string {
	PruneFieldsLock.RLock()
	fields, ok := PruneFields[resource]
	PruneFieldsLock.RUnlock()
	if !ok {
		return []string{}
	}

	return fields
}

// SetPruneFields accepts a config.Resource that will be used to determine the fields that should
// be pruned accordingly.
func SetPruneFields(resources []config.Resource) {
	PruneFieldsLock.Lock()
	newResourceMap := map[string][]string{}
	for _, resource := range resources {
		newResourceMap[resource.Name] = resource.PruneFields
	}

	PruneFields = newResourceMap
	PruneFieldsLock.Unlock()
	glog.Infof("fields to prune loaded as: %s", PruneFields)
}
