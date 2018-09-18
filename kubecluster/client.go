// Copyright 2018 Heptio
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package kubecluster

import (
	"github.com/golang/glog"
	crdClient "k8s.io/apiextensions-apiserver/examples/client-go/pkg/client/clientset/versioned/typed/cr/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	config *rest.config
	err    error
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
		glog.Infof("--kubeconfig flag specified, attempting to load kubeconfig from %s", cPath)

		config, err = clientcmd.BuildConfigFromFlags("", cPath)
		if err != nil {
			return nil, err
		}
		glog.Infof("client being created to communicate with API server at %s", config.Host)

		return kubernetes.NewForConfig(config)
	}

	// attempt to create client from in-cluster config (the service account associated witht the pod)
	glog.Infof(`no --kubeconfig flag specified, loading Kubernetes service account assigned to pod 
		located at /var/run/secrets/kubernetes.io/serviceaccount/.`)

	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	glog.Infof("client being created to communicate with API server at %s", config.Host)

	return kubernetes.NewForConfig(config)
}

func NewCRDClient() *crdClient.Clientset {
	return crdClient.NewForConfig(config)
}
