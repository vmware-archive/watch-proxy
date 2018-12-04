// Copyright 2018 Heptio
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubecluster

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/oklog/ulid"

	"github.com/pborman/uuid"

	"github.com/heptio/quartermaster/config"
	vs_clientset "github.com/heptio/quartermaster/custom/client/clientset/versioned"
	"github.com/heptio/quartermaster/inventory"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

// Takes a snapshot of the cluster and creates the Cluster object
var (
	uuidLock sync.Mutex
	lastUUID uuid.UUID
	cluster  inventory.Cluster
)

// StartWatchers creates all informers needed to satisfy the desired objects to keep a watch on, as
// described in the config. It expects a kubernetes client for which rest clients will be derived,
// the configuration of quartermaster, and a processing queue where all objects that triggered
// events will be stored.
func StartWatchers(client *kubernetes.Clientset, vsClient *vs_clientset.Clientset, c config.Config,
	processorQueue workqueue.RateLimitingInterface) InformerClients {
	// loop through list of resources to watch and startup watchers
	// for those resources.
	resources := []config.Resource{}
	// decide if we want to load the full set of resources. Usually at startup
	// or just start up new watchers after config change
	if len(c.NewResources) > 0 {
		resources = c.NewResources
	} else {
		resources = c.ResourcesWatch
	}

	// list of InformerClients to be instantiated.
	ics := InformerClients{}

	// start the watchers
	for _, resource := range resources {
		ic, err := NewInformerClient(client, vsClient, resource.Name, "", processorQueue, c)

		if err != nil {
			glog.Errorf("failure to create client to listen for %s objects. They will not be "+
				"watched", resource)
			continue
		}
		go ic.Start()
		// add started InformerClient to list of InformerClients
		ics = append(ics, ic)
	}

	// return the map of done channels so we can stop things later if need be
	glog.Infof("watchers started for the following resource types: %s", ics)
	return ics
}

// StopWatchers looks through the StaleResources (watcher to stop)  inside of the provided
// config. It sends a stop signal to all running watchers in this list and returns a list of all
// the watchers (or InformerClients) it stopped.
func StopWatchers(ics InformerClients, config config.Config) InformerClients {
	// holds list of deleted InformerClients
	stoppedIcs := InformerClients{}
	// loop through list of stale resource types and stop those watchers
	for _, staleRes := range config.StaleResources {
		glog.Infof("stopping %s", staleRes)
		// if the watcher is found stop it otherwise do nothing
		if stoppedIc := ics.FindInformerClient(staleRes.Name); stoppedIc != nil {
			stoppedIc.Stop()
			stoppedIcs = append(stoppedIcs, stoppedIc)
		}
	}
	glog.Infof("watchers stopped for the following resource types: %s", stoppedIcs)
	return stoppedIcs
}

// GetK8sVersion - get the version of k8s running on the cluster
func GetK8sVersion(client *kubernetes.Clientset) string {
	version, err := client.DiscoveryClient.ServerVersion()
	if err != nil {
		glog.Infof("could not get server version. error: %s", err)
	}
	return version.GitVersion

}

// GetClusterName provides a unique cluster name
func GetClusterName(client *kubernetes.Clientset) string {
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		glog.Errorf("could not get nodes. error: %s", err)
	}
	node := nodes.Items[0]
	if clusterName, ok := node.ObjectMeta.Labels["cluster-name"]; ok {
		return clusterName
	} else {
		t := time.Unix(1000000, 0)
		entropy := rand.New(rand.NewSource(t.UnixNano()))
		clusterName := fmt.Sprint(ulid.MustNew(ulid.Timestamp(t), entropy))
		return clusterName
	}

}
