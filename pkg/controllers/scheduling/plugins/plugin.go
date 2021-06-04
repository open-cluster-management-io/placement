package plugins

import (
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
)

type Plugin interface {
	ResetWithPlacement(pl *clusterapiv1alpha1.Placement)
}

type Filter interface {
	Filter(clusters []*clusterapiv1.ManagedCluster) []*clusterapiv1.ManagedCluster
}

type Prioritizer interface {
	Score(clusters []*clusterapiv1.ManagedCluster) map[string]float64
}
