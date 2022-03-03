package plugins

import (
	"context"
	"math"
	"time"

	"k8s.io/client-go/tools/events"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterlisterv1alpha1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1alpha1"
	clusterlisterv1beta1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta1"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

const (
	// MaxClusterScore is the maximum score a Prioritizer plugin is expected to return.
	MaxClusterScore int64 = 100

	// MinClusterScore is the minimum score a Prioritizer plugin is expected to return.
	MinClusterScore int64 = -100

	// MaxTotalScore is the maximum total score.
	MaxTotalScore int64 = math.MaxInt64
)

// Plugin is the parent type for all the scheduling plugins.
type Plugin interface {
	Name() string
	// Set is to set the placement for the current scheduling.
	Description() string
	// RequeueAfter returns the requeue time interval of the placement
	RequeueAfter(ctx context.Context, placement *clusterapiv1beta1.Placement) PluginRequeueResult
}

// Fitler defines a filter plugin that filter unsatisfied cluster.
type Filter interface {
	Plugin

	// Filter returns a list of clusters satisfying the certain condition.
	Filter(ctx context.Context, placement *clusterapiv1beta1.Placement, clusters []*clusterapiv1.ManagedCluster) PluginFilterResult
}

// Prioritizer defines a prioritizer plugin that score each cluster. The score is normalized
// as a floating betwween 0 and 1.
type Prioritizer interface {
	Plugin

	// Score gives the score to a list of the clusters, it returns a map with the key as
	// the cluster name.
	Score(ctx context.Context, placement *clusterapiv1beta1.Placement, clusters []*clusterapiv1.ManagedCluster) PluginScoreResult
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization.
type Handle interface {
	// DecisionLister lists all decisions
	DecisionLister() clusterlisterv1beta1.PlacementDecisionLister

	// ScoreLister lists all AddOnPlacementScores
	ScoreLister() clusterlisterv1alpha1.AddOnPlacementScoreLister

	// ClusterClient returns the cluster client
	ClusterClient() clusterclient.Interface

	// EventRecorder returns an event recorder.
	EventRecorder() events.EventRecorder
}

// PluginFilterResult contains the details of a filter plugin result.
type PluginFilterResult struct {
	// Filtered contains the filtered ManagedCluster.
	Filtered []*clusterapiv1.ManagedCluster
	// Err contains the filter plugin error message.
	Err error
}

// PluginScoreResult contains the details of a score plugin result.
type PluginScoreResult struct {
	// Scores contains the ManagedCluster scores.
	Scores map[string]int64
	// Err contains the score plugin error message.
	Err error
}

// PluginRequeueResult contains the requeue result of a placement.
type PluginRequeueResult struct {
	// RequeueTime contains the expect requeue time.
	RequeueTime *time.Time
	// Reasons contains the message about requeueTime generation.
	Reasons []string
	// Err contains the plugin requeue error message.
	Err error
}
