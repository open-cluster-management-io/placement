package tainttoleration

import (
	"context"
	"errors"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"open-cluster-management.io/placement/pkg/plugins"
)

var _ plugins.Filter = &TaintToleration{}
var TolerationClock = (clock.Clock)(clock.RealClock{})

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

func (pl *TaintToleration) Filter(ctx context.Context, placement *clusterapiv1beta1.Placement, clusters []*clusterapiv1.ManagedCluster) plugins.PluginFilterResult {
	if len(clusters) == 0 {
		return plugins.PluginFilterResult{
			Filtered: clusters,
		}
	}

	// do validation on each toleration and return error if necessary
	for _, toleration := range placement.Spec.Tolerations {
		if len(toleration.Key) == 0 && toleration.Operator != clusterapiv1beta1.TolerationOpExists {
			return plugins.PluginFilterResult{
				Err: errors.New("If the key is empty, operator must be Exists.\n"),
			}
		}
		if toleration.Operator == clusterapiv1beta1.TolerationOpExists && len(toleration.Value) > 0 {
			return plugins.PluginFilterResult{
				Err: errors.New("If the operator is Exists, the value should be empty.\n"),
			}
		}
	}

	existingDecisions := getDecisions(pl.handle, placement)

	// filter the clusters
	matched := []*clusterapiv1.ManagedCluster{}
	for _, cluster := range clusters {
		if isClusterTolerated(cluster, placement.Spec.Tolerations, existingDecisions.Has(cluster.Name)) {
			matched = append(matched, cluster)
		}
	}

	return plugins.PluginFilterResult{
		Filtered: matched,
	}
}

func (pl *TaintToleration) RequeueAfter(ctx context.Context, placement *clusterapiv1beta1.Placement) plugins.PluginRequeueResult {
	return plugins.PluginRequeueResult{}
}

// isClusterTolerated returns true if a cluster is tolerated by the given toleration array
func isClusterTolerated(cluster *clusterapiv1.ManagedCluster, tolerations []clusterapiv1beta1.Toleration, inDecision bool) bool {
	for _, taint := range cluster.Spec.Taints {
		if !isTaintTolerated(taint, tolerations, inDecision) {
			return false
		}
	}
	return true
}

// isTaintTolerated returns true if a taint is tolerated by the given toleration array
func isTaintTolerated(taint clusterapiv1.Taint, tolerations []clusterapiv1beta1.Toleration, inDecision bool) bool {
	if taint.Effect == clusterapiv1.TaintEffectPreferNoSelect {
		return true
	}

	if (taint.Effect == clusterapiv1.TaintEffectNoSelectIfNew) && inDecision {
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
func isTolerated(taint clusterapiv1.Taint, toleration clusterapiv1beta1.Toleration) bool {
	if len(toleration.Effect) > 0 && toleration.Effect != taint.Effect {
		return false
	}

	if len(toleration.Key) > 0 && toleration.Key != taint.Key {
		return false
	}

	taintMatched := false

	switch toleration.Operator {
	// empty operator means Equal
	case "", clusterapiv1beta1.TolerationOpEqual:
		taintMatched = (toleration.Value == taint.Value)
	case clusterapiv1beta1.TolerationOpExists:
		taintMatched = true
	}

	return taintMatched && isTolerationTimeExpired(taint, toleration)
}

// isTolerationTimeExpired returns true if TolerationSeconds is nil or not expired
func isTolerationTimeExpired(taint clusterapiv1.Taint, toleration clusterapiv1beta1.Toleration) bool {
	// TolerationSeconds is nil means it never expire
	if toleration.TolerationSeconds == nil {
		return true
	}

	// timeDiff = toleration.TolerationSeconds - (now - taint.TimeAdded)
	timeDiff := time.Duration(*toleration.TolerationSeconds)*time.Second - TolerationClock.Since(taint.TimeAdded.Time)
	return timeDiff > 0
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
