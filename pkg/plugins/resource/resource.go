package resource

import (
	"context"
	"math"

	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	"open-cluster-management.io/placement/pkg/plugins"
)

const (
	placementLabel = "cluster.open-cluster-management.io/placement"
	description    = `
	Resource prioritizer makes the scheduling decisions based on cluster resource utilization rate.
	The cluster with the least resource utilization rate is given the highest score.
	The source could be cpu, memory or both of them.
	`
)

var _ plugins.Prioritizer = &Resource{}

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
	minScore := int64(100)
	maxScore := int64(0)

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

		minScore = min(minScore, scores[cluster.Name])
		maxScore = max(maxScore, scores[cluster.Name])
	}

	normalizeScore(minScore, maxScore, clusters, scores)
}

func mostAllocatable(placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster, scores map[string]int64) {
	minscore := int64(100)
	maxscore := int64(0)

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

		minscore = min(minscore, scores[cluster.Name])
		maxscore = max(maxscore, scores[cluster.Name])
	}

	normalizeScore(minscore, maxscore, clusters, scores)
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

func normalizeScore(minScore, maxScore int64, clusters []*clusterapiv1.ManagedCluster, scores map[string]int64) {
	// normalize the score and ensure the value falls in the range between 0 and 100.
	if minScore > maxScore {
		return
	}

	// normalized = (score - min(score)) * 100 / (max(score) - min(score))
	for _, cluster := range clusters {
		if minScore < maxScore {
			scores[cluster.Name] = (scores[cluster.Name] - minScore) * 100 / (maxScore - minScore)
		}
		if minScore == maxScore {
			if minScore == 0 {
				scores[cluster.Name] = 0
			} else {
				scores[cluster.Name] = 100
			}
		}
	}
}

func min(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}

func max(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}
