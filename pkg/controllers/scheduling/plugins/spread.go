package plugins

import (
	"fmt"

	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Spread struct {
	pl                   *clusterapiv1alpha1.Placement
	selectedClusters     *sets.String
	topologyKeyPairCount map[topologyPair]int
}

type topologyPair struct {
	mode  string
	key   string
	value string
}

func (a *Spread) ResetWithPlacement(pl *clusterapiv1alpha1.Placement) {
	a.pl = pl
	a.topologyKeyPairCount = map[topologyPair]int{}
}

func (a *Spread) Filter(clusters []*clusterapiv1.ManagedCluster) []*clusterapiv1.ManagedCluster {
	if len(a.pl.Spec.SpreadPolicy.SpreadConstraints) == 0 {
		return clusters
	}

	filtered := []*clusterapiv1.ManagedCluster{}
	filteredConstraints := []clusterapiv1alpha1.SpreadConstraintsTerm{}
	for _, con := range a.pl.Spec.SpreadPolicy.SpreadConstraints {
		if con.WhenUnsatisfiable == "DoNotSelect" {
			filteredConstraints = append(filteredConstraints, con)
		}
	}
	selectedClusters := []*clusterapiv1.ManagedCluster{}
	candidateClusters := []*clusterapiv1.ManagedCluster{}
	for _, cluster := range clusters {
		claims := getClusterClaims(cluster)
		if a.selectedClusters.Has(cluster.Name) {
			selectedClusters = append(selectedClusters, cluster)
			continue
		}

		if !matchSpreadConstraints(cluster.Labels, filteredConstraints, "Label") {
			continue
		}
		if !matchSpreadConstraints(claims, filteredConstraints, "Claim") {
			continue
		}
		candidateClusters = append(candidateClusters, cluster)
	}

	// Calculate spread count
	for _, cluster := range selectedClusters {
		claims := getClusterClaims(cluster)
		a.updateTopologyPairCount(cluster.Labels, filteredConstraints, "Label")
		a.updateTopologyPairCount(claims, filteredConstraints, "Claim")
	}

	for _, cluster := range candidateClusters {
		claims := getClusterClaims(cluster)
		doNotSelect := false
		for _, c := range filteredConstraints {
			_, labelskew, _ := a.calSkew(cluster.Labels, c, "Label")
			_, claimskew, _ := a.calSkew(claims, c, "Claim")
			if labelskew+claimskew > int(c.MaxSkew) {
				doNotSelect = true
			}
		}

		if !doNotSelect {
			filtered = append(filtered, cluster)
		}
	}
	return filtered
}

func (a *Spread) Score(clusters []*clusterapiv1.ManagedCluster) map[string]float64 {
	scores := map[string]float64{}
	return scores
}

func (a *Spread) calSkew(vals map[string]string, c clusterapiv1alpha1.SpreadConstraintsTerm, mode string) (topologyPair, int, error) {
	if c.TopologyKeyType != mode {
		return topologyPair{}, 0, fmt.Errorf("unmatched mode")
	}
	label, ok := vals[c.TopologyKey]
	if !ok {
		return topologyPair{}, 0, fmt.Errorf("unmatched key")
	}
	pair := topologyPair{mode: c.TopologyKeyType, key: c.TopologyKey, value: label}
	skew, exist := a.topologyKeyPairCount[pair]
	if !exist {
		return pair, 1, nil
	}

	return pair, skew + 1, nil
}

func (a *Spread) updateTopologyPairCount(
	vals map[string]string,
	constraints []clusterapiv1alpha1.SpreadConstraintsTerm,
	mode string) {
	for _, c := range constraints {
		pair, skew, err := a.calSkew(vals, c, mode)
		if err != nil {
			continue
		}
		a.topologyKeyPairCount[pair] = skew
	}
}

func matchSpreadConstraints(vals map[string]string, constraints []clusterapiv1alpha1.SpreadConstraintsTerm, mode string) bool {
	for _, c := range constraints {
		if mode != c.TopologyKeyType {
			continue
		}
		if _, ok := vals[c.TopologyKey]; !ok {
			return false
		}
	}
	return true
}
