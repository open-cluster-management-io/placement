package plugins

import (
	clusterlisterv1alpha1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1alpha1"
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

type Balance struct {
	pl                      *clusterapiv1alpha1.Placement
	placementDecisionLister clusterlisterv1alpha1.PlacementDecisionLister
}

func (a *Balance) ResetWithPlacement(pl *clusterapiv1alpha1.Placement) {
	a.pl = pl
}

func (a *Balance) Score(clusters []*clusterapiv1.ManagedCluster) map[string]float64 {
	scores := map[string]float64{}
	for _, cluster := range clusters {
		scores[cluster.Name] = 1.0
	}

	decisions, err := a.placementDecisionLister.List(labels.Everything())
	if err != nil {
		return scores
	}

	maxCount := 0
	decisionCount := map[string]int{}
	for _, decision := range decisions {
		if decision.Labels[placementLabel] == a.pl.Name {
			continue
		}
		for _, d := range decision.Status.Decisions {
			decisionCount[d.ClusterName] = decisionCount[d.ClusterName] + 1
			if decisionCount[d.ClusterName] > maxCount {
				maxCount = decisionCount[d.ClusterName]
			}
		}
	}

	for clusterName := range scores {
		if count, ok := decisionCount[clusterName]; ok {
			scores[clusterName] = scores[clusterName] - float64(count)/float64(maxCount)
		}
	}
	return scores
}
