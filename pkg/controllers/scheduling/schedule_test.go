package scheduling

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	clusterfake "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	testinghelpers "open-cluster-management.io/placement/pkg/helpers/testing"
)

func TestSchedule(t *testing.T) {
	clusterSetName := "clusterSets"
	placementNamespace := "ns1"
	placementName := "placement1"

	cases := []struct {
		name                 string
		placement            *clusterapiv1alpha1.Placement
		initObjs             []runtime.Object
		clusters             []*clusterapiv1.ManagedCluster
		decisions            []runtime.Object
		expectedFilterResult []FilterResult
		expectedScoreResult  []PrioritizerResult
		expectedDecisions    []clusterapiv1alpha1.ClusterDecision
		expectedUnScheduled  int
	}{
		{
			name:      "new placement satisfied",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
			},
			decisions: []runtime.Object{},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			expectedUnScheduled: 0,
		},
		{
			name:      "new placement unsatisfied",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(3).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
			},
			decisions: []runtime.Object{},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			expectedUnScheduled: 2,
		},
		{
			name:      "placement with all decisions scheduled",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(2).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster1", "cluster2").Build(),
			},
			decisions: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster1", "cluster2").Build(),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster3").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1", "cluster2", "cluster3"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 100, "cluster3": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 100, "cluster3": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			expectedUnScheduled: 0,
		},
		{
			name:      "placement with empty Prioritizer Policy",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithPrioritizerPolicy("").Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
			},
			decisions: []runtime.Object{},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			expectedUnScheduled: 0,
		},
		{
			name:      "placement with additive Prioritizer Policy",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(2).WithPrioritizerPolicy("Additive").WithPrioritizerConfig("Balance", 3).WithPrioritizerConfig("ResourceAllocatableMemory", 1).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).WithResource(clusterapiv1.ResourceMemory, "100", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, clusterSetName).WithResource(clusterapiv1.ResourceMemory, "50", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithLabel(clusterSetLabel, clusterSetName).WithResource(clusterapiv1.ResourceMemory, "0", "100").Build(),
			},
			decisions: []runtime.Object{},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1", "cluster2", "cluster3"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 3,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 100, "cluster3": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0, "cluster2": 0, "cluster3": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 0, "cluster3": -100},
				},
			},
			expectedUnScheduled: 0,
		},
		{
			name:      "placement with exact Prioritizer Policy",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(2).WithPrioritizerPolicy("Exact").WithPrioritizerConfig("Balance", 3).WithPrioritizerConfig("ResourceAllocatableMemory", 1).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).WithResource(clusterapiv1.ResourceMemory, "100", "100").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, clusterSetName).WithResource(clusterapiv1.ResourceMemory, "50", "100").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithLabel(clusterSetLabel, clusterSetName).WithResource(clusterapiv1.ResourceMemory, "0", "100").Build(),
			},
			decisions: []runtime.Object{},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1", "cluster2", "cluster3"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 3,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 100, "cluster3": 100},
				},
				{
					Name:   "Steady",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 0, "cluster3": -100},
				},
			},
			expectedUnScheduled: 0,
		},
		{
			name:      "placement with part of decisions scheduled",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(4).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster1").Build(),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			decisions: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster1").Build(),
			},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster1", "cluster2"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 100, "cluster2": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			expectedUnScheduled: 2,
		},
		{
			name:      "schedule to cluster with least decisions",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(1).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName("others", 1)).
					WithDecisions("cluster1", "cluster2").Build(),
			},
			decisions: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName("others", 1)).
					WithDecisions("cluster1", "cluster2").Build(),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster3").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster3"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster3", "cluster1", "cluster2"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": -100, "cluster2": -100, "cluster3": 100},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0, "cluster2": 0, "cluster3": 0},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			expectedUnScheduled: 0,
		},
		{
			name:      "do not schedule to other cluster even with least decisions",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(1).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet(clusterSetName),
				testinghelpers.NewClusterSetBinding(placementNamespace, clusterSetName),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName("others", 1)).
					WithDecisions("cluster3", "cluster2").Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName("others", 2)).
					WithDecisions("cluster2", "cluster1").Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster3").Build(),
			},
			decisions: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName("others", 1)).
					WithDecisions("cluster3", "cluster2").Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName("others", 2)).
					WithDecisions("cluster2", "cluster1").Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster3").Build(),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, clusterSetName).Build(),
				testinghelpers.NewManagedCluster("cluster3").WithLabel(clusterSetLabel, clusterSetName).Build(),
			},
			expectedDecisions: []clusterapiv1alpha1.ClusterDecision{
				{ClusterName: "cluster3"},
			},
			expectedFilterResult: []FilterResult{
				{
					Name:             "Predicate",
					FilteredClusters: []string{"cluster3", "cluster1", "cluster2"},
				},
			},
			expectedScoreResult: []PrioritizerResult{
				{
					Name:   "Balance",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0, "cluster2": -100, "cluster3": 0},
				},
				{
					Name:   "Steady",
					Weight: 1,
					Scores: PrioritizerScore{"cluster1": 0, "cluster2": 0, "cluster3": 100},
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 0,
					Scores: nil,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 0,
					Scores: nil,
				},
			},
			expectedUnScheduled: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.initObjs = append(c.initObjs, c.placement)
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)
			kubeClient := &kubernetes.Clientset{}
			s := NewPluginScheduler(testinghelpers.NewFakePluginHandle(t, clusterClient, kubeClient, c.initObjs...))
			result, err := s.Schedule(
				context.TODO(),
				c.placement,
				c.clusters,
			)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(result.Decisions(), c.expectedDecisions) {
				t.Errorf("expected %v scheduled, but got %v", c.expectedDecisions, result.Decisions())
			}
			if result.NumOfUnscheduled() != c.expectedUnScheduled {
				t.Errorf("expected %d unscheduled, but got %d", c.expectedUnScheduled, result.NumOfUnscheduled())
			}

			actual, _ := json.Marshal(result.FilterResults())
			expected, _ := json.Marshal(c.expectedFilterResult)
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("expected filter results %v, but got %v", string(expected), string(actual))
			}

			actual, _ = json.Marshal(result.PrioritizerResults())
			expected, _ = json.Marshal(c.expectedScoreResult)
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("expected score results %v, but got %v", string(expected), string(actual))
			}
		})
	}
}

func placementDecisionName(placementName string, index int) string {
	return fmt.Sprintf("%s-decision-%d", placementName, index)
}

func TestFilterResults(t *testing.T) {

}
