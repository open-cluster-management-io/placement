// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
	v1 "open-cluster-management.io/api/cluster/v1"
	v1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	v1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=cluster.open-cluster-management.io, Version=v1
	case v1.SchemeGroupVersion.WithResource("managedclusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1().ManagedClusters().Informer()}, nil

		// Group=cluster.open-cluster-management.io, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithResource("clusterclaims"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().ClusterClaims().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("managedclusterscalars"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().ManagedClusterScalars().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("managedclustersets"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().ManagedClusterSets().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("managedclustersetbindings"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().ManagedClusterSetBindings().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("placements"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().Placements().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("placementdecisions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().PlacementDecisions().Informer()}, nil

		// Group=cluster.open-cluster-management.io, Version=v1beta1
	case v1beta1.SchemeGroupVersion.WithResource("managedclustersets"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1beta1().ManagedClusterSets().Informer()}, nil
	case v1beta1.SchemeGroupVersion.WithResource("managedclustersetbindings"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1beta1().ManagedClusterSetBindings().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
