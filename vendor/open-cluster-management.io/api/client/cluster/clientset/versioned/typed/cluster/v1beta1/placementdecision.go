// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	scheme "open-cluster-management.io/api/client/cluster/clientset/versioned/scheme"
	v1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// PlacementDecisionsGetter has a method to return a PlacementDecisionInterface.
// A group's client should implement this interface.
type PlacementDecisionsGetter interface {
	PlacementDecisions(namespace string) PlacementDecisionInterface
}

// PlacementDecisionInterface has methods to work with PlacementDecision resources.
type PlacementDecisionInterface interface {
	Create(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.CreateOptions) (*v1beta1.PlacementDecision, error)
	Update(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.UpdateOptions) (*v1beta1.PlacementDecision, error)
	UpdateStatus(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.UpdateOptions) (*v1beta1.PlacementDecision, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.PlacementDecision, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.PlacementDecisionList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.PlacementDecision, err error)
	PlacementDecisionExpansion
}

// placementDecisions implements PlacementDecisionInterface
type placementDecisions struct {
	client rest.Interface
	ns     string
}

// newPlacementDecisions returns a PlacementDecisions
func newPlacementDecisions(c *ClusterV1beta1Client, namespace string) *placementDecisions {
	return &placementDecisions{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the placementDecision, and returns the corresponding placementDecision object, and an error if there is any.
func (c *placementDecisions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.PlacementDecision, err error) {
	result = &v1beta1.PlacementDecision{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("placementdecisions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PlacementDecisions that match those selectors.
func (c *placementDecisions) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.PlacementDecisionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.PlacementDecisionList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("placementdecisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested placementDecisions.
func (c *placementDecisions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("placementdecisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a placementDecision and creates it.  Returns the server's representation of the placementDecision, and an error, if there is any.
func (c *placementDecisions) Create(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.CreateOptions) (result *v1beta1.PlacementDecision, err error) {
	result = &v1beta1.PlacementDecision{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("placementdecisions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(placementDecision).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a placementDecision and updates it. Returns the server's representation of the placementDecision, and an error, if there is any.
func (c *placementDecisions) Update(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.UpdateOptions) (result *v1beta1.PlacementDecision, err error) {
	result = &v1beta1.PlacementDecision{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("placementdecisions").
		Name(placementDecision.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(placementDecision).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *placementDecisions) UpdateStatus(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.UpdateOptions) (result *v1beta1.PlacementDecision, err error) {
	result = &v1beta1.PlacementDecision{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("placementdecisions").
		Name(placementDecision.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(placementDecision).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the placementDecision and deletes it. Returns an error if one occurs.
func (c *placementDecisions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("placementdecisions").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *placementDecisions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("placementdecisions").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched placementDecision.
func (c *placementDecisions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.PlacementDecision, err error) {
	result = &v1beta1.PlacementDecision{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("placementdecisions").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
