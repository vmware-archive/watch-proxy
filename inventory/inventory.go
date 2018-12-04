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

package inventory

import (
	"k8s.io/apimachinery/pkg/types"
)

// Inventory - Type for the whole inventory
type Inventory struct {
	Clusters []Cluster
}

// Cluster - Type that describes cluster info
type Cluster struct {
	UID         types.UID
	Version     string
	Name        string
	Namespaces  []Namespace
	Deployments map[string][]Deployment
	Pods        map[string][]Pod
}

type Namespace struct {
	UID   types.UID
	Name  string
	Event string
	Kind  string
}

// Deployment - type that describes deployment info
type Deployment struct {
	UID             types.UID
	Name            string
	Namespace       string
	Labels          map[string]string
	ReplicasDesired int32
	Event           string
	Kind            string
}

// Pod - type that describes pod info
type Pod struct {
	UID       types.UID
	Name      string
	Namespace string
	Labels    map[string]string
	Images    []string
	Event     string
	Kind      string
}
