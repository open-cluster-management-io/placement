package plugins

import (
	"context"
	"math"

	"k8s.io/client-go/tools/events"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterlisterv1alpha1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1alpha1"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
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
}

// Fitler defines a filter plugin that filter unsatisfied cluster.
type Filter interface {
	Plugin

	// Filter returns a list of clusters satisfying the certain condition.
	Filter(ctx context.Context, placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) ([]*clusterapiv1.ManagedCluster, error)
}

// Prioritizer defines a prioritizer plugin that score each cluster. The score is normalized
// as a floating betwween 0 and 1.
type Prioritizer interface {
	Plugin

	// Score gives the score to a list of the clusters, it returns a map with the key as
	// the cluster name.
	Score(ctx context.Context, placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) (map[string]int64, error)
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization.
type Handle interface {
	// ListDecisionsInPlacment lists all decisions
	DecisionLister() clusterlisterv1alpha1.PlacementDecisionLister

	// ClusterClient returns the cluster client
	ClusterClient() clusterclient.Interface

	// EventRecorder returns an event recorder.
	EventRecorder() events.EventRecorder
}
