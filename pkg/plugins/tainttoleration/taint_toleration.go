package tainttoleration

import (
	"context"
	"reflect"

	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	"open-cluster-management.io/placement/pkg/plugins"
)

var _ plugins.Filter = &TaintToleration{}

const description = "TaintToleration is a plugin that checks if a placement tolerates a managed cluster's taints"

type TaintToleration struct{}

func New(handle plugins.Handle) *TaintToleration {
	return &TaintToleration{}
}

func (p *TaintToleration) Name() string {
	return reflect.TypeOf(*p).Name()
}

func (pl *TaintToleration) Description() string {
	return description
}

func (pl *TaintToleration) Filter(ctx context.Context, placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) ([]*clusterapiv1.ManagedCluster, error) {

	if len(placement.Spec.Tolerations) == 0 {
		return clusters, nil
	}
	if len(clusters) == 0 {
		return clusters, nil
	}

	filterPredicate := func(t *clusterapiv1.Taint) bool {
		// PodToleratesNodeTaints is only interested in NoSchedule and NoExecute taints.
		return t.Effect == clusterapiv1.TaintEffectNoSelect || t.Effect == clusterapiv1.TaintEffectNoSelectIfNew
	}

	// match cluster with selectors one by one
	matched := []*clusterapiv1.ManagedCluster{}
	for _, cluster := range clusters {
		isUntolerated := IfUntolerated(cluster.Spec.Taints, placement.Spec.Tolerations, filterPredicate)
		if isUntolerated {
			continue
		}
		matched = append(matched, cluster)
	}

	return matched, nil
}

type taintsFilterFunc func(*clusterapiv1.Taint) bool

// IfUntolerated checks if the given tolerations tolerates
// all the filtered taints, and returns the first taint without a toleration
// Returns true if there is an untolerated taint
// Returns false if all taints are tolerated
func IfUntolerated(taints []clusterapiv1.Taint, tolerations []clusterapiv1alpha1.Toleration, inclusionFilter taintsFilterFunc) bool {
	filteredTaints := getFilteredTaints(taints, inclusionFilter)
	for _, taint := range filteredTaints {
		if !TolerationsTolerateTaint(tolerations, &taint) {
			return true
		}
	}
	return false
}

// getFilteredTaints returns a list of taints satisfying the filter predicate
func getFilteredTaints(taints []clusterapiv1.Taint, inclusionFilter taintsFilterFunc) []clusterapiv1.Taint {
	if inclusionFilter == nil {
		return taints
	}
	filteredTaints := []clusterapiv1.Taint{}
	for _, taint := range taints {
		if !inclusionFilter(&taint) {
			continue
		}
		filteredTaints = append(filteredTaints, taint)
	}
	return filteredTaints
}

// TolerationsTolerateTaint checks if taint is tolerated by any of the tolerations.
func TolerationsTolerateTaint(tolerations []clusterapiv1alpha1.Toleration, taint *clusterapiv1.Taint) bool {
	for i := range tolerations {
		if ToleratesTaint(taint, &tolerations[i]) {
			return true
		}
	}
	return false
}

// ToleratesTaint checks if the toleration tolerates the taint.
// The matching follows the rules below:
// (1) Empty toleration.effect means to match all taint effects,
//     otherwise taint effect must equal to toleration.effect.
// (2) If toleration.operator is 'Exists', it means to match all taint values.
// (3) Empty toleration.key means to match all taint keys.
//     If toleration.key is empty, toleration.operator must be 'Exists';
//     this combination means to match all taint values and all taint keys.
func ToleratesTaint(taint *clusterapiv1.Taint, t *clusterapiv1alpha1.Toleration) bool {
	if len(t.Effect) > 0 && t.Effect != taint.Effect {
		return false
	}

	if len(t.Key) > 0 && t.Key != taint.Key {
		return false
	}

	switch t.Operator {
	// empty operator means Equal
	case "", clusterapiv1alpha1.TolerationOpEqual:
		return t.Value == taint.Value
	case clusterapiv1alpha1.TolerationOpExists:
		return true
	default:
		return false
	}
}
