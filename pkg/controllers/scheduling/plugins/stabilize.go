package plugins

import (
	clusterlisterv1alpha1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1alpha1"
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
)

const placementLabel = "cluster.open-cluster-management.io/placement"

type Stabilize struct {
	pl                      *clusterapiv1alpha1.Placement
	placementDecisionLister clusterlisterv1alpha1.PlacementDecisionLister
}

func (a *Stabilize) ResetWithPlacement(pl *clusterapiv1alpha1.Placement) {
	a.pl = pl
}

func (a *Stabilize) Score(clusters []*clusterapiv1.ManagedCluster) map[string]float64 {
	// query placementdecisions with label selector
	scores := map[string]float64{}
	requirement, _ := labels.NewRequirement(placementLabel, selection.Equals, []string{a.pl.Name})
	labelSelector := labels.NewSelector().Add(*requirement)
	decisions, err := a.placementDecisionLister.PlacementDecisions(a.pl.Namespace).List(labelSelector)
	if err != nil {
		return scores
	}

	existingDecisions := sets.String{}
	for _, decision := range decisions {
		for _, d := range decision.Status.Decisions {
			existingDecisions.Insert(d.ClusterName)
		}
	}

	for _, cluster := range clusters {
		if existingDecisions.Has(cluster.Name) {
			scores[cluster.Name] = 1.0
		} else {
			scores[cluster.Name] = 0.0
		}
	}

	return scores
}
