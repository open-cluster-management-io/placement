package steady

import (
	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"open-cluster-management.io/placement/pkg/plugins"
)

const (
	placementLabel = "cluster.open-cluster-management.io/placement"
	description    = `
	Steady prioritizer ensure the existing decision is stabilized. The clusters that existing decisions
	choose are given the highest score while the clusters with no existing decisions are given the lowest
	score.
	`
)

var _ plugins.Prioritizer = &Steady{}

type Steady struct {
	handle plugins.Handle
}

func New(handle plugins.Handle) *Steady {
	return &Steady{
		handle: handle,
	}
}

func (s *Steady) Name() string {
	return reflect.TypeOf(*s).Name()
}

func (s *Steady) Description() string {
	return description
}

func (s *Steady) Score(
	ctx context.Context, placement *clusterapiv1beta1.Placement, clusters []*clusterapiv1.ManagedCluster) (map[string]int64, *time.Duration, error) {
	// query placementdecisions with label selector
	scores := map[string]int64{}
	requirement, err := labels.NewRequirement(placementLabel, selection.Equals, []string{placement.Name})

	if err != nil {
		return nil, nil, err
	}

	labelSelector := labels.NewSelector().Add(*requirement)
	decisions, err := s.handle.DecisionLister().PlacementDecisions(placement.Namespace).List(labelSelector)

	if err != nil {
		return nil, nil, err
	}

	existingDecisions := sets.String{}
	for _, decision := range decisions {
		for _, d := range decision.Status.Decisions {
			existingDecisions.Insert(d.ClusterName)
		}
	}

	for _, cluster := range clusters {
		if existingDecisions.Has(cluster.Name) {
			scores[cluster.Name] = plugins.MaxClusterScore
		} else {
			scores[cluster.Name] = 0
		}
	}

	return scores, nil, nil
}
