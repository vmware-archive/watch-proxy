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
