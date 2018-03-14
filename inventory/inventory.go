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
	Namespaces  []string
	Deployments map[string][]Deployment
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
