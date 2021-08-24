package resource

import (
	"context"
	"math"

	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	"open-cluster-management.io/placement/pkg/plugins"
	"open-cluster-management.io/placement/pkg/plugins/util"
)

const (
	placementLabel = "cluster.open-cluster-management.io/placement"
	description    = `
	Resource prioritizer makes the scheduling decisions based on the resource allocatable/capacity 
	or allocatable of managed clusters.
    The clusters that has the most allocatable/capacity or allocatable are given the highest score, 
	while the least is given the lowest score.
	`
)

var _ plugins.Prioritizer = &Resource{}
var scale = int64(2)

type ResourceValueList map[clusterapiv1.ResourceName]float64

type Resource struct {
	handle plugins.Handle
}

func New(handle plugins.Handle) *Resource {
	return &Resource{handle: handle}
}

func (b *Resource) Name() string {
	return "resource"
}

func (b *Resource) Description() string {
	return description
}

func (b *Resource) Score(ctx context.Context, placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) (map[string]int64, error) {
	scores := map[string]int64{}
	if placement.Spec.ClusterResourcePreference == nil {
		return scores, nil
	}

	switch {
	case placement.Spec.ClusterResourcePreference.Type == clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatable:
		mostAllocatable(placement, clusters, scores)
	case placement.Spec.ClusterResourcePreference.Type == clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatableToCapacityRatio:
		mostAllocatableToCapacityRatio(placement, clusters, scores)
	}

	return scores, nil
}

func mostAllocatableToCapacityRatio(placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster, scores map[string]int64) {
	minscore := int64(math.MaxInt64)
	maxscore := int64(math.MinInt64)

	for _, cluster := range clusters {
		allocatable, capacity := getClusterResources(cluster)

		// score = sum(resource_x_allocatable / resource_x_capacity))
		for _, clusterResource := range placement.Spec.ClusterResourcePreference.ClusterResources {
			resourceName := clusterapiv1.ResourceName(clusterResource.ResourceName)

			allocatable, hasAllocatable := allocatable[resourceName]
			capacity, hasCapacity := capacity[resourceName]
			if hasAllocatable && hasCapacity && capacity != 0 {
				scores[cluster.Name] += int64(allocatable * 100.0 / capacity)
			}
		}

		minscore = util.Min(minscore, scores[cluster.Name])
		maxscore = util.Max(maxscore, scores[cluster.Name])
	}

	util.NormalizeScore(minscore, maxscore, scale, clusters, scores)
}

func mostAllocatable(placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster, scores map[string]int64) {
	minscore := int64(math.MaxInt64)
	maxscore := int64(math.MinInt64)

	minAllocatableResources, maxAllocatableResources := getClustersMinMaxAllocatableResources(clusters)

	for _, cluster := range clusters {
		allocatable, _ := getClusterResources(cluster)

		// score = sum((resource_x_allocatable - min(resource_x_allocatable)) / (max(resource_x_allocatable) - min(resource_x_allocatable))
		for _, clusterResource := range placement.Spec.ClusterResourcePreference.ClusterResources {
			resourceName := clusterapiv1.ResourceName(clusterResource.ResourceName)

			minAllocatable, hasMin := minAllocatableResources[resourceName]
			maxAllocatable, hasMax := maxAllocatableResources[resourceName]
			allocatable, hasAllocatable := allocatable[resourceName]

			if hasMin && hasMax && hasAllocatable && (maxAllocatable-minAllocatable) != 0 {
				scores[cluster.Name] += int64((allocatable - minAllocatable) * 100.0 / (maxAllocatable - minAllocatable))
			}
		}

		minscore = util.Min(minscore, scores[cluster.Name])
		maxscore = util.Max(maxscore, scores[cluster.Name])
	}

	util.NormalizeScore(minscore, maxscore, scale, clusters, scores)
}

func getClusterResources(cluster *clusterapiv1.ManagedCluster) (allocatable, capacity ResourceValueList) {
	allocatable = make(ResourceValueList)
	capacity = make(ResourceValueList)

	for k, v := range cluster.Status.Allocatable {
		allocatable[k] = v.AsApproximateFloat64()
	}

	for k, v := range cluster.Status.Capacity {
		capacity[k] = v.AsApproximateFloat64()
	}

	return allocatable, capacity
}

func getClustersMinMaxAllocatableResources(clusters []*clusterapiv1.ManagedCluster) (min, max ResourceValueList) {
	min = make(ResourceValueList)
	max = make(ResourceValueList)

	for _, cluster := range clusters {
		allocatable, _ := getClusterResources(cluster)
		for k, v := range allocatable {
			_, hasMin := min[k]
			_, hasMax := max[k]
			if hasMin {
				min[k] = math.Min(min[k], v)
			} else {
				min[k] = v
			}
			if hasMax {
				max[k] = math.Max(max[k], v)
			} else {
				max[k] = v
			}
		}
	}

	return min, max
}
