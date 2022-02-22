package tainttoleration

import (
	"context"
	"errors"
	"reflect"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"open-cluster-management.io/placement/pkg/plugins"
)

var _ plugins.Filter = &TaintToleration{}

const (
	placementLabel = "cluster.open-cluster-management.io/placement"
	description    = "TaintToleration is a plugin that checks if a placement tolerates a managed cluster's taints"
)

type TaintToleration struct {
	handle plugins.Handle
}

func New(handle plugins.Handle) *TaintToleration {
	return &TaintToleration{
		handle: handle,
	}
}

func (p *TaintToleration) Name() string {
	return reflect.TypeOf(*p).Name()
}

func (pl *TaintToleration) Description() string {
	return description
}

func (pl *TaintToleration) Filter(ctx context.Context, placement *clusterapiv1beta1.Placement, clusters []*clusterapiv1.ManagedCluster) ([]*clusterapiv1.ManagedCluster, error) {
	if len(clusters) == 0 {
		return clusters, nil
	}

	// do validation on each toleration and return error if necessary
	for _, toleration := range placement.Spec.Tolerations {
		if len(toleration.Key) == 0 && toleration.Operator != clusterapiv1beta1.TolerationOpExists {
			return nil, errors.New("If the key is empty, operator must be Exists.\n")
		}
		if toleration.Operator == clusterapiv1beta1.TolerationOpExists && len(toleration.Value) > 0 {
			return nil, errors.New("If the operator is Exists, the value should be empty.\n")
		}
		//		if toleration.TolerationSeconds != nil && toleration.Effect != clusterapiv1.TaintEffectNoSelect && toleration.Effect != clusterapiv1.TaintEffectPreferNoSelect {
		//			klog.Warningf("TolerationSeconds would be ignored if Effect is not NoSelect/PreferNoSelect.")
		//		}
	}

	existingDecisions := getDecisions(pl.handle, placement)

	matched := []*clusterapiv1.ManagedCluster{}
	for _, cluster := range clusters {
		if isClusterTolerated(cluster, placement.Spec.Tolerations, existingDecisions.Has(cluster.Name)) {
			matched = append(matched, cluster)
		}
	}
	return matched, nil
}

// TODO: TolerationSeconds
// isClusterTolerated returns true if a cluster is tolerated by the given toleration array
func isClusterTolerated(cluster *clusterapiv1.ManagedCluster, tolerations []clusterapiv1beta1.Toleration, exist bool) bool {
	for _, taint := range cluster.Spec.Taints {
		if !isTaintTolerated(taint, tolerations, exist) {
			return false
		}
	}
	return true
}

// isTaintTolerated returns true if a taint is tolerated by the given toleration array
func isTaintTolerated(taint clusterapiv1.Taint, tolerations []clusterapiv1beta1.Toleration, exist bool) bool {
	for _, toleration := range tolerations {
		if isToleratedByEffect(taint, toleration, exist) || isTolerated(taint, toleration) {
			return true
		}
	}
	return false
}

// isToleratedByEffect returns true if effect is NoSelectIfNew and cluster exists in placement decision
// or the effect is PreferNoSelect
func isToleratedByEffect(taint clusterapiv1.Taint, toleration clusterapiv1beta1.Toleration, exist bool) bool {
	if len(toleration.Effect) > 0 && toleration.Effect != taint.Effect {
		return false
	}

	switch taint.Effect {
	case clusterapiv1.TaintEffectNoSelect:
		return false
	case clusterapiv1.TaintEffectNoSelectIfNew:
		return exist
	case clusterapiv1.TaintEffectPreferNoSelect:
		return true
	default:
		return false
	}
}

// isTolerated returns true if a taint is tolerated by the given toleration
func isTolerated(taint clusterapiv1.Taint, toleration clusterapiv1beta1.Toleration) bool {
	switch toleration.Operator {
	// empty operator means Equal
	case "", clusterapiv1beta1.TolerationOpEqual:
		return (toleration.Key == taint.Key) && (toleration.Value == taint.Value)
	case clusterapiv1beta1.TolerationOpExists:
		// empty key means to match all values and all keys.
		return (toleration.Key == "") || (toleration.Key == taint.Key)
	default:
		return false
	}
}

func getDecisions(handle plugins.Handle, placement *clusterapiv1beta1.Placement) sets.String {
	existingDecisions := sets.String{}

	// query placementdecisions with label selector
	requirement, err := labels.NewRequirement(placementLabel, selection.Equals, []string{placement.Name})
	if err != nil {
		return existingDecisions
	}

	labelSelector := labels.NewSelector().Add(*requirement)
	decisions, err := handle.DecisionLister().PlacementDecisions(placement.Namespace).List(labelSelector)
	if err != nil {
		return existingDecisions
	}

	for _, decision := range decisions {
		for _, d := range decision.Status.Decisions {
			existingDecisions.Insert(d.ClusterName)
		}
	}

	return existingDecisions
}
