package fake

import (
	clientset "github.com/heptio/quartermaster/custom/client/clientset/versioned"
	virtualservicev1alpha3 "github.com/heptio/quartermaster/custom/client/clientset/versioned/typed/virtualservice/v1alpha3"
	fakevirtualservicev1alpha3 "github.com/heptio/quartermaster/custom/client/clientset/versioned/typed/virtualservice/v1alpha3/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
)

// NewSimpleClientset returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewSimpleClientset(objects ...runtime.Object) *Clientset {
	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	fakePtr := testing.Fake{}
	fakePtr.AddReactor("*", "*", testing.ObjectReaction(o))
	fakePtr.AddWatchReactor("*", testing.DefaultWatchReactor(watch.NewFake(), nil))

	return &Clientset{fakePtr, &fakediscovery.FakeDiscovery{Fake: &fakePtr}}
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type Clientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

var _ clientset.Interface = &Clientset{}

// VirtualserviceV1alpha3 retrieves the VirtualserviceV1alpha3Client
func (c *Clientset) VirtualserviceV1alpha3() virtualservicev1alpha3.VirtualserviceV1alpha3Interface {
	return &fakevirtualservicev1alpha3.FakeVirtualserviceV1alpha3{Fake: &c.Fake}
}

// Virtualservice retrieves the VirtualserviceV1alpha3Client
func (c *Clientset) Virtualservice() virtualservicev1alpha3.VirtualserviceV1alpha3Interface {
	return &fakevirtualservicev1alpha3.FakeVirtualserviceV1alpha3{Fake: &c.Fake}
}
