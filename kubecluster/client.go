package kubecluster

import (
	"github.com/golang/glog"
	vs_clientset "github.com/heptio/quartermaster/custom/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewK8SClient returns a kubernetes client based on a passed kubeconfig path. If a kubeconfig
// path is not passed (this is normal), the service account associated with the controller will
// be loaded automatically.
func NewK8sClient(cPath string) (*kubernetes.Clientset, *vs_clientset.Clientset, error) {
	// clientcmd.BuildConfigFromFlags will call rest.InClusterConfig when the kubeconfigPath
	// passed into it is empty. However it logs an inappropriate warning could give the operator
	// unnecessary concern. Thus, we'll only call this when there is a kubeconfig explicitly
	// passed.
	if cPath != "" {
		glog.Infof("--kubeconfig flag specified, attempting to load kubeconfig from %s", cPath)

		config, err := clientcmd.BuildConfigFromFlags("", cPath)
		if err != nil {
			return nil, nil, err
		}
		glog.Infof("client being created to communicate with API server at %s", config.Host)

		cs, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, nil, err
		}

		vcs, err := vs_clientset.NewForConfig(config)
		if err != nil {
			return nil, nil, err
		}

		return cs, vcs, err
	}

	// attempt to create client from in-cluster config (the service account associated witht the pod)
	glog.Infof(`no --kubeconfig flag specified, loading Kubernetes service account assigned to pod 
		located at /var/run/secrets/kubernetes.io/serviceaccount/.`)

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}
	glog.Infof("client being created to communicate with API server at %s", config.Host)

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	vcs, err := vs_clientset.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return cs, vcs, err
}
