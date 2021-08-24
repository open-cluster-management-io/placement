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
		acpu, ccpu, amem, cmem := getClusterResources(cluster)

		// score = sum(resource_x_allocatable / resource_x_capacity))
		for _, r := range placement.Spec.ClusterResourcePreference.ClusterResources {
			if r.ResourceName == clusterapiv1alpha1.ClusterResourceNameNameCPU && ccpu != 0 {
				scores[cluster.Name] += int64(acpu * 100.0 / ccpu)
			}
			if r.ResourceName == clusterapiv1alpha1.ClusterResourceNameMemory && cmem != 0 {
				scores[cluster.Name] += int64(amem * 100.0 / cmem)
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

	mincpu, maxcpu, minmem, maxmem := getClustersMinMaxAllocatableResources(clusters)

	for _, cluster := range clusters {
		acpu, _, amem, _ := getClusterResources(cluster)

		// score = sum((resource_x_allocatable - min(resource_x_allocatable)) / (max(resource_x_allocatable) - min(resource_x_allocatable))
		for _, r := range placement.Spec.ClusterResourcePreference.ClusterResources {
			if r.ResourceName == clusterapiv1alpha1.ClusterResourceNameNameCPU && (maxcpu-mincpu) != 0 {
				scores[cluster.Name] += int64((acpu - mincpu) * 100.0 / (maxcpu - mincpu))
			}
			if r.ResourceName == clusterapiv1alpha1.ClusterResourceNameMemory && (maxmem-minmem) != 0 {
				scores[cluster.Name] += int64((amem - minmem) * 100.0 / (maxmem - minmem))
			}
		}

		minscore = util.Min(minscore, scores[cluster.Name])
		maxscore = util.Max(maxscore, scores[cluster.Name])
	}

	util.NormalizeScore(minscore, maxscore, scale, clusters, scores)
}

func getClusterResources(cluster *clusterapiv1.ManagedCluster) (acpu, ccpu, amem, cmem float64) {
	arcpu := cluster.Status.Allocatable[clusterapiv1.ResourceCPU]
	crcpu := cluster.Status.Capacity[clusterapiv1.ResourceCPU]
	armem := cluster.Status.Allocatable[clusterapiv1.ResourceMemory]
	crmem := cluster.Status.Capacity[clusterapiv1.ResourceMemory]

	return arcpu.AsApproximateFloat64(), crcpu.AsApproximateFloat64(), armem.AsApproximateFloat64(), crmem.AsApproximateFloat64()
}

func getClustersMinMaxAllocatableResources(clusters []*clusterapiv1.ManagedCluster) (mincpu, maxcpu, minmem, maxmem float64) {
	for _, cluster := range clusters {
		acpu, _, amem, _ := getClusterResources(cluster)
		mincpu = math.Min(mincpu, acpu)
		maxcpu = math.Max(maxcpu, acpu)
		minmem = math.Min(minmem, amem)
		maxmem = math.Max(maxmem, amem)
	}

	return mincpu, maxcpu, minmem, maxmem
}
