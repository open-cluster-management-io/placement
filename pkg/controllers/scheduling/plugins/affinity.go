package plugins

import (
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Affinity struct {
	pl *clusterapiv1alpha1.Placement
}

func (a *Affinity) ResetWithPlacement(pl *clusterapiv1alpha1.Placement) {
	a.pl = pl
}

func (a *Affinity) Filter(clusters []*clusterapiv1.ManagedCluster) []*clusterapiv1.ManagedCluster {
	if len(a.pl.Spec.Affinity.ClusterAffinity) == 0 {
		return clusters
	}

	filtered := []*clusterapiv1.ManagedCluster{}
	for _, cluster := range clusters {
		if score := a.scoreWithAffinity(cluster, "DoNotSelect"); score != 1.0 {
			continue
		}
		filtered = append(filtered, cluster)
	}

	return filtered
}

func (a *Affinity) Score(clusters []*clusterapiv1.ManagedCluster) map[string]float64 {
	scores := map[string]float64{}
	for _, cluster := range clusters {
		scores[cluster.Name] = a.scoreWithAffinity(cluster, "SelectAnyway")
	}
	return scores
}

func (a *Affinity) scoreWithAffinity(cluster *clusterapiv1.ManagedCluster, mode string) float64 {
	var score, sum float64
	claims := getClusterClaims(cluster)
	for _, affinity := range a.pl.Spec.Affinity.ClusterAffinity {
		if affinity.WhenUnsatisfiable != mode {
			continue
		}
		weight := 1.0
		if affinity.Weight != 0 {
			weight = float64(affinity.Weight)
		}
		sum = sum + weight
		labelSelector, err := convertLabelSelector(affinity.LabelSelector)
		if err != nil {
			continue
		}
		claimSelector, err := convertClaimSelector(affinity.ClaimSelector)
		if err != nil {
			continue
		}
		if labelSelector.Matches(labels.Set(cluster.Labels)) &&
			claimSelector.Matches(labels.Set(claims)) {
			score = score + weight
		}
	}

	if sum == 0.0 {
		return 1.0
	}

	return score / sum
}

// convertLabelSelector converts metav1.LabelSelector to labels.Selector
func convertLabelSelector(labelSelector metav1.LabelSelector) (labels.Selector, error) {
	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return labels.Nothing(), err
	}

	return selector, nil
}

// convertClaimSelector converts ClusterClaimSelector to labels.Selector
func convertClaimSelector(clusterClaimSelector clusterapiv1alpha1.ClusterClaimSelector) (labels.Selector, error) {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: clusterClaimSelector.MatchExpressions,
	})
	if err != nil {
		return labels.Nothing(), err
	}

	return selector, nil
}

// getClusterClaims returns a map containing cluster claims from the status of cluster
func getClusterClaims(cluster *clusterapiv1.ManagedCluster) map[string]string {
	claims := map[string]string{}
	for _, claim := range cluster.Status.ClusterClaims {
		claims[claim.Name] = claim.Value
	}
	return claims
}
