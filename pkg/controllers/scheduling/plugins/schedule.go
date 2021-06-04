package plugins

import (
	"sort"

	clusterlisterv1alpha1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1alpha1"
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Scheduler struct {
	plugins  []Plugin
	selected sets.String
}

func NewScheduler(placementDecisionLister clusterlisterv1alpha1.PlacementDecisionLister) *Scheduler {
	scehduler := &Scheduler{}
	scehduler.plugins = []Plugin{
		&Affinity{},
		&Spread{selectedClusters: &scehduler.selected},
		&Balance{placementDecisionLister: placementDecisionLister},
		&Stabilize{placementDecisionLister: placementDecisionLister},
	}

	return scehduler
}

func (s *Scheduler) Schedule(
	pl *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) sets.String {
	// Tell plugin the current placement to process
	for _, p := range s.plugins {
		p.ResetWithPlacement(pl)
	}
	s.selected = sets.String{}

	// Determine the number of decisions
	numDecision := len(clusters)
	if pl.Spec.NumberOfClusters != nil {
		numDecision = int(*pl.Spec.NumberOfClusters)
	}

	// We now sechedule for each decision using clusters as the candidate
	for i := 0; i < numDecision; i++ {
		selectedCluster := s.scheduleOne(pl, clusters, s.selected)
		if selectedCluster != nil {
			s.selected.Insert(selectedCluster.Name)
		}
	}

	return s.selected
}

func (s *Scheduler) scheduleOne(
	pl *clusterapiv1alpha1.Placement,
	clusters []*clusterapiv1.ManagedCluster,
	selected sets.String,
) *clusterapiv1.ManagedCluster {
	filtered := clusters
	// Filter the cluster at first.
	// TODO we can parallelize here.
	for _, p := range s.plugins {
		if filter, ok := p.(Filter); ok {
			filtered = filter.Filter(filtered)
		}
	}

	// If NumberOfClusters is not set, it is not necessary to
	// score
	if pl.Spec.NumberOfClusters == nil {
		for _, c := range filtered {
			if !selected.Has(c.Name) {
				return c
			}
		}
		return nil
	}

	// Score the cluster
	scoreSum := map[string]float64{}
	for _, cluster := range filtered {
		scoreSum[cluster.Name] = 0.0
	}
	for _, p := range s.plugins {
		proritizer, ok := p.(Prioritizer)
		if !ok {
			continue
		}
		score := proritizer.Score(filtered)
		for name := range score {
			scoreSum[name] = scoreSum[name] + score[name]
		}
	}

	// Sort cluster by score
	sort.SliceStable(filtered, func(i, j int) bool {
		switch {
		case scoreSum[clusters[i].Name] != scoreSum[clusters[j].Name]:
			return scoreSum[clusters[i].Name] > scoreSum[clusters[j].Name]
		default:
			return clusters[i].Name < clusters[j].Name
		}
	})

	for _, c := range filtered {
		if !selected.Has(c.Name) {
			return c
		}
	}
	return nil
}
