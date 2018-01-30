package inventory

// Inventory - Type for the whole inventory
type Inventory struct {
	Clusters []Cluster
}

// Cluster - Type that describes cluster info
type Cluster struct {
	Version     string
	Namespaces  []string
	Deployments map[string][]Deployment
}

type Namespace struct {
	Name  string
	Event string
	Kind  string
}

// Deployment - type that describes deployment info
type Deployment struct {
	Name            string
	Namespace       string
	Labels          map[string]string
	ReplicasDesired int32
	Event           string
	Kind            string
}

// Pod - type that describes pod info
type Pod struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Images    []string
	Event     string
	Kind      string
}
