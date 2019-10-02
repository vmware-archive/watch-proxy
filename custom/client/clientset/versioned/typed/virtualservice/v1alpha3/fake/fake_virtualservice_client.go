// Copyright 2018-2019 VMware, Inc. 
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	v1alpha3 "github.com/heptio/quartermaster/custom/client/clientset/versioned/typed/virtualservice/v1alpha3"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeVirtualserviceV1alpha3 struct {
	*testing.Fake
}

func (c *FakeVirtualserviceV1alpha3) VirtualServices(namespace string) v1alpha3.VirtualServiceInterface {
	return &FakeVirtualServices{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeVirtualserviceV1alpha3) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
