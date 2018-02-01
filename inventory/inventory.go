package inventory

import (
	"encoding/json"
	"fmt"

	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
)

// Inventory - Type for the whole inventory
// type Inventory struct {
// 	Clusters []Cluster
// }

// Cluster - Type that describes cluster info
type Cluster struct {
	Version     string
	Namespaces  []Namespace
	Deployments map[string][]Deployment
	Pods        map[string][]Pod
	Images      map[string][]string
}

type Namespace struct {
	Name string
	Kind string
}

func (n *Namespace) DecodeNamespace(file []byte) {
	// fmt.Printf("%+v", string(file))
	k8sNS := v1.Namespace{}
	if err := json.Unmarshal(file, &k8sNS); err != nil {
		fmt.Println("error decoding fileData:", err)
	}
	n.Name = k8sNS.ObjectMeta.Name
	n.Kind = k8sNS.Kind
}

// Deployment - type that describes deployment info
type Deployment struct {
	Name            string
	Namespace       string
	Labels          map[string]string
	ReplicasDesired int32
	Kind            string
}

func (dep *Deployment) DecodeDeployment(file []byte) {
	k8sDep := v1beta2.Deployment{}
	if err := json.Unmarshal(file, &k8sDep); err != nil {
		fmt.Println("error decoding fileData:", err)
	}
	dep.Name = k8sDep.ObjectMeta.Name
	dep.Kind = k8sDep.Kind
	dep.Namespace = k8sDep.ObjectMeta.Namespace
	dep.Labels = k8sDep.ObjectMeta.Labels
	dep.ReplicasDesired = *k8sDep.Spec.Replicas
}

func MapDeps(deps []Deployment) map[string][]Deployment {
	depMap := make(map[string][]Deployment)
	for _, dep := range deps {
		depMap[dep.Namespace] = append(depMap[dep.Namespace], dep)
	}

	return depMap

}

// Pod - type that describes pod info
type Pod struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Images    []string
	Kind      string
}

func (pod *Pod) DecodePod(file []byte) {
	k8sPod := v1.Pod{}
	if err := json.Unmarshal(file, &k8sPod); err != nil {
		fmt.Println("error decoding fileData:", err)
	}
	pod.Name = k8sPod.ObjectMeta.Name
	pod.Kind = k8sPod.Kind
	pod.Namespace = k8sPod.ObjectMeta.Namespace
	pod.Labels = k8sPod.ObjectMeta.Labels
	pod.imagesFromContainers(k8sPod.Spec.Containers)

}

func MapPods(pods []Pod) map[string][]Pod {
	podMap := make(map[string][]Pod)
	for _, pod := range pods {
		podMap[pod.Namespace] = append(podMap[pod.Namespace], pod)
	}

	return podMap

}

func (p *Pod) imagesFromContainers(containers []v1.Container) {

	images := []string{}
	for _, cont := range containers {
		p.Images = append(images, cont.Image)
	}

}
