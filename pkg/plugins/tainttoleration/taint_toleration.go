package tainttoleration

import (
	"context"
	"errors"
	"fmt"
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
	if len(clusters) == 0 {
		return clusters, nil
	}
	// do validation on each toleration and return error if necessary
	for _, toleration := range placement.Spec.Tolerations {
		if len(toleration.Key) == 0 && toleration.Operator != clusterapiv1alpha1.TolerationOpExists {
			return nil, errors.New("If the key is empty, operator must be Exists.\n")
		}
		if toleration.Operator == clusterapiv1alpha1.TolerationOpExists && len(toleration.Value) > 0 {
			return nil, errors.New("If the operator is Exists, the value should be empty.\n")
		}
		if toleration.TolerationSeconds != nil && toleration.Effect != clusterapiv1.TaintEffectNoSelect && toleration.Effect != clusterapiv1.TaintEffectPreferNoSelect {
			fmt.Println("Warning: TolerationSeconds would be ignored if Effect is not NoSelect/PreferNoSelect.\n")
		}
	}
	// If the placement has no toleration, all clusters with taint should be filtered out.
	if len(placement.Spec.Tolerations) == 0 {
		clusterWithoutTaints := []*clusterapiv1.ManagedCluster{}
		for _, cluster := range clusters {
			if len(cluster.Spec.Taints) == 0 {
				clusterWithoutTaints = append(clusterWithoutTaints, cluster)
			}
		}
		return clusterWithoutTaints, nil
	}

	matched := []*clusterapiv1.ManagedCluster{}
	for _, cluster := range clusters {
		if isClusterTolerated(cluster, placement.Spec.Tolerations) {
			matched = append(matched, cluster)
		}
	}
	return matched, nil
}

// isClusterTolerated returns true if a cluster is tolerated by the given toleration array
func isClusterTolerated(cluster *clusterapiv1.ManagedCluster, tolerations []clusterapiv1alpha1.Toleration) bool {
	for _, taint := range cluster.Spec.Taints {
		if !isTaintTolerated(taint, tolerations) {
			return false
		}
	}
	return true
}

// isTaintTolerated returns true if a taint is tolerated by the given toleration array
func isTaintTolerated(taint clusterapiv1.Taint, tolerations []clusterapiv1alpha1.Toleration) bool {
	if taint.Effect == clusterapiv1.TaintEffectPreferNoSelect {
		return true
	}

	for _, toleration := range tolerations {
		if isTolerated(taint, toleration) {
			return true
		}
	}
	return false
}

// isTolerated returns true if a taint is tolerated by the given toleration
func isTolerated(taint clusterapiv1.Taint, toleration clusterapiv1alpha1.Toleration) bool {
	if len(toleration.Effect) > 0 && toleration.Effect != taint.Effect {
		return false
	}

	if len(toleration.Key) > 0 && toleration.Key != taint.Key {
		return false
	}

	switch toleration.Operator {
	// empty operator means Equal
	case "", clusterapiv1alpha1.TolerationOpEqual:
		return toleration.Value == taint.Value
	case clusterapiv1alpha1.TolerationOpExists:
		return true
	default:
		return false
	}
}
