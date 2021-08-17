package resource

import (
	"context"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	testinghelpers "open-cluster-management.io/placement/pkg/helpers/testing"
)

func TestScoreClusterWithResource(t *testing.T) {
	cases := []struct {
		name              string
		placement         *clusterapiv1alpha1.Placement
		clusters          []*clusterapiv1.ManagedCluster
		existingDecisions []runtime.Object
		expectedScores    map[string]int64
	}{
		{
			name:      "scores with ClusterResourcePreference type is MostAllocatableToCapacityRatio",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatableToCapacityRatio).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("10", "100", "100", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("9", "10", "90", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("8", "10", "80", "100").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 100, "cluster3": 71},
		},
		{
			name:      "scores with ClusterResourcePreference type is MostAllocatable",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatable).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("10", "100", "100", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("9", "10", "90", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("8", "10", "80", "100").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 100, "cluster2": 50, "cluster3": 0},
		},
		{
			name:      "scores with ClusterResourcePreference type is nil",
			placement: testinghelpers.NewPlacement("test", "test").Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("10", "10", "100", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("10", "10", "100", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("10", "10", "100", "100").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{},
		},
		{
			name:      "scores when ClusterResource allocatable is 0 and type is MostAllocatableToCapacityRatio",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatableToCapacityRatio).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("0", "10", "0", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("0", "10", "0", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("0", "10", "0", "100").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 0, "cluster3": 0},
		},
		{
			name:      "scores when ClusterResource allocatable is 0 and type is MostAllocatable",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatable).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("0", "10", "0", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("0", "10", "0", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("0", "10", "0", "100").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 0, "cluster3": 0},
		},
		{
			name:      "scores when ClusterResource capacity is 0 and type is MostAllocatableToCapacityRatio",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatableToCapacityRatio).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("0", "0", "0", "0").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("0", "0", "0", "0").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("0", "0", "0", "0").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 0, "cluster3": 0},
		},
		{
			name:      "scores when ClusterResource capacity is 0 and type is MostAllocatable",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatable).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithResource("0", "0", "0", "0").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithResource("0", "0", "0", "0").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithResource("0", "0", "0", "0").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 0, "cluster3": 0},
		},
		{
			name:      "scores when no ClusterResource and type is MostAllocatableToCapacityRatio",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatableToCapacityRatio).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").Build(),
				testinghelpers.NewManagedCluster("cluster2").Build(),
				testinghelpers.NewManagedCluster("cluster3").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 0, "cluster3": 0},
		},
		{
			name:      "scores when no ClusterResource and type is MostAllocatable",
			placement: testinghelpers.NewPlacement("test", "test").WithClusterResourcePreference(clusterapiv1alpha1.ClusterResourcePreferenceTypeMostAllocatable).Build(),
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").Build(),
				testinghelpers.NewManagedCluster("cluster2").Build(),
				testinghelpers.NewManagedCluster("cluster3").Build(),
			},
			existingDecisions: []runtime.Object{},
			expectedScores:    map[string]int64{"cluster1": 0, "cluster2": 0, "cluster3": 0},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resource := &Resource{
				handle: testinghelpers.NewFakePluginHandle(t, nil, c.existingDecisions...),
			}

			scores, err := resource.Score(context.TODO(), c.placement, c.clusters)
			if err != nil {
				t.Errorf("Expect no error, but got %v", err)
			}

			if !apiequality.Semantic.DeepEqual(scores, c.expectedScores) {
				t.Errorf("Expect score %v, but got %v", c.expectedScores, scores)
			}
		})
	}
}
