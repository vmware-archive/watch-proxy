package kubecluster

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewK8SClient returns a kubernetes client based on a passed kubeconfig path. If a kubeconfig
// path is not passed (this is normal), the service account associated with the controller will
// be loaded automatically.
func NewK8sClient(cPath string) (*kubernetes.Clientset, error) {
	// clientcmd.BuildConfigFromFlags will call rest.InClusterConfig when the kubeconfigPath
	// passed into it is empty. However it logs an inappropriate warning could give the operator
	// unnecessary concern. Thus, we'll only call this when there is a kubeconfig explicitly
	// passed.
	if cPath != "" {
		log.Printf("--kubeconfig flag specified, attempting to load kubeconfig from %s", cPath)

		config, err := clientcmd.BuildConfigFromFlags("", cPath)
		if err != nil {
			return nil, err
		}
		log.Printf("client being created to communicate with API server at %s", config.Host)

		return kubernetes.NewForConfig(config)
	}

	// attempt to create client from in-cluster config (the service account associated witht the pod)
	log.Println(`no --kubeconfig flag specified, loading Kubernetes service account assigned to pod 
		located at /var/run/secrets/kubernetes.io/serviceaccount/.`)

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	log.Printf("client being created to communicate with API server at %s", config.Host)

	return kubernetes.NewForConfig(config)
}
