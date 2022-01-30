package scheduling

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	clienttesting "k8s.io/client-go/testing"
	kevents "k8s.io/client-go/tools/events"
	clusterfake "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	testinghelpers "open-cluster-management.io/placement/pkg/helpers/testing"
)

type testScheduler struct {
	result ScheduleResult
}

func (s *testScheduler) PrePare(ctx context.Context,
	placement *clusterapiv1beta1.Placement,
) (SchedulePrioritizers, error) {
	return nil, nil
}

func (s *testScheduler) Schedule(ctx context.Context,
	placement *clusterapiv1beta1.Placement,
	clusters []*clusterapiv1.ManagedCluster,
	schedulePrioritizers SchedulePrioritizers,
) (ScheduleResult, error) {
	return s.result, nil
}

func TestSchedulingController_sync(t *testing.T) {
	placementNamespace := "ns1"
	placementName := "placement1"

	cases := []struct {
		name            string
		placement       *clusterapiv1beta1.Placement
		initObjs        []runtime.Object
		scheduleResult  *scheduleResult
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:      "placement satisfied",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet("clusterset1"),
				testinghelpers.NewClusterSetBinding(placementNamespace, "clusterset1"),
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, "clusterset1").Build(),
			},
			scheduleResult: &scheduleResult{
				feasibleClusters: []*clusterapiv1.ManagedCluster{
					testinghelpers.NewManagedCluster("cluster1").Build(),
					testinghelpers.NewManagedCluster("cluster2").Build(),
					testinghelpers.NewManagedCluster("cluster3").Build(),
				},
				scheduledDecisions: []clusterapiv1beta1.ClusterDecision{
					{ClusterName: "cluster1"},
					{ClusterName: "cluster2"},
					{ClusterName: "cluster3"},
				},
				unscheduledDecisions: 0,
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				// check if PlacementDecision has been updated
				testinghelpers.AssertActions(t, actions, "create", "update", "update")
				// check if Placement has been updated
				actual := actions[2].(clienttesting.UpdateActionImpl).Object
				placement, ok := actual.(*clusterapiv1beta1.Placement)
				if !ok {
					t.Errorf("expected Placement was updated")
				}

				if placement.Status.NumberOfSelectedClusters != int32(3) {
					t.Errorf("expecte %d cluster selected, but got %d", 3, placement.Status.NumberOfSelectedClusters)
				}
				ok = testinghelpers.HasCondition(
					placement.Status.Conditions,
					clusterapiv1beta1.PlacementConditionSatisfied,
					"AllDecisionsScheduled",
					metav1.ConditionTrue,
				)
				if !ok {
					t.Errorf("unexpected condition %v", placement.Status.Conditions)
				}
				ok = testinghelpers.HasCondition(
					placement.Status.Conditions,
					"PlacementMisconfigured",
					"CorrectConfiguration",
					metav1.ConditionFalse,
				)
				if !ok {
					t.Errorf("unexpected condition %v", placement.Status.Conditions)
				}
			},
		},
		{
			name:      "placement unsatisfied",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet("clusterset1"),
				testinghelpers.NewClusterSetBinding(placementNamespace, "clusterset1"),
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, "clusterset1").Build(),
			},
			scheduleResult: &scheduleResult{
				feasibleClusters: []*clusterapiv1.ManagedCluster{
					testinghelpers.NewManagedCluster("cluster1").Build(),
					testinghelpers.NewManagedCluster("cluster2").Build(),
					testinghelpers.NewManagedCluster("cluster3").Build(),
				},
				scheduledDecisions: []clusterapiv1beta1.ClusterDecision{
					{ClusterName: "cluster1"},
					{ClusterName: "cluster2"},
					{ClusterName: "cluster3"},
				},
				unscheduledDecisions: 1,
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				// check if PlacementDecision has been updated
				testinghelpers.AssertActions(t, actions, "create", "update", "update")
				// check if Placement has been updated
				actual := actions[2].(clienttesting.UpdateActionImpl).Object
				placement, ok := actual.(*clusterapiv1beta1.Placement)
				if !ok {
					t.Errorf("expected Placement was updated")
				}

				if placement.Status.NumberOfSelectedClusters != int32(3) {
					t.Errorf("expecte %d cluster selected, but got %d", 3, placement.Status.NumberOfSelectedClusters)
				}
				ok = testinghelpers.HasCondition(
					placement.Status.Conditions,
					clusterapiv1beta1.PlacementConditionSatisfied,
					"NotAllDecisionsScheduled",
					metav1.ConditionFalse,
				)
				if !ok {
					t.Errorf("unexpected condition %v", placement.Status.Conditions)
				}
			},
		},
		{
			name:      "placement missing managedclustersetbindings",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).Build(),
			scheduleResult: &scheduleResult{
				feasibleClusters:     []*clusterapiv1.ManagedCluster{},
				scheduledDecisions:   []clusterapiv1beta1.ClusterDecision{},
				unscheduledDecisions: 0,
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				// check if PlacementDecision has been updated
				testinghelpers.AssertActions(t, actions, "update")
				// check if Placement has been updated
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placement, ok := actual.(*clusterapiv1beta1.Placement)
				if !ok {
					t.Errorf("expected Placement was updated")
				}

				if placement.Status.NumberOfSelectedClusters != int32(0) {
					t.Errorf("expecte %d cluster selected, but got %d", 0, placement.Status.NumberOfSelectedClusters)
				}
				ok = testinghelpers.HasCondition(
					placement.Status.Conditions,
					clusterapiv1beta1.PlacementConditionSatisfied,
					"NoManagedClusterSetBindings",
					metav1.ConditionFalse,
				)
				if !ok {
					t.Errorf("unexpected condition %v", placement.Status.Conditions)
				}
			},
		},
		{
			name: "placement status not changed",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).
				WithNumOfSelectedClusters(3).WithSatisfiedCondition(3, 0).WithMisconfiguredCondition("").Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet("clusterset1"),
				testinghelpers.NewClusterSetBinding(placementNamespace, "clusterset1"),
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, "clusterset1").Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster1", "cluster2", "cluster3").Build(),
			},
			scheduleResult: &scheduleResult{
				feasibleClusters: []*clusterapiv1.ManagedCluster{
					testinghelpers.NewManagedCluster("cluster1").Build(),
					testinghelpers.NewManagedCluster("cluster2").Build(),
					testinghelpers.NewManagedCluster("cluster3").Build(),
				},
				scheduledDecisions: []clusterapiv1beta1.ClusterDecision{
					{ClusterName: "cluster1"},
					{ClusterName: "cluster2"},
					{ClusterName: "cluster3"},
				},
				unscheduledDecisions: 0,
			},
			validateActions: testinghelpers.AssertNoActions,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.initObjs = append(c.initObjs, c.placement)
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)
			clusterInformerFactory := testinghelpers.NewClusterInformerFactory(clusterClient, c.initObjs...)
			s := &testScheduler{result: c.scheduleResult}

			ctrl := schedulingController{
				clusterClient:           clusterClient,
				clusterLister:           clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				clusterSetLister:        clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets().Lister(),
				clusterSetBindingLister: clusterInformerFactory.Cluster().V1beta1().ManagedClusterSetBindings().Lister(),
				placementLister:         clusterInformerFactory.Cluster().V1beta1().Placements().Lister(),
				placementDecisionLister: clusterInformerFactory.Cluster().V1beta1().PlacementDecisions().Lister(),
				scheduler:               s,
				recorder:                kevents.NewFakeRecorder(100),
			}

			sysCtx := testinghelpers.NewFakeSyncContext(t, c.placement.Namespace+"/"+c.placement.Name)
			syncErr := ctrl.sync(context.TODO(), sysCtx)
			if syncErr != nil {
				t.Errorf("unexpected err: %v", syncErr)
			}
			c.validateActions(t, clusterClient.Actions())
		})
	}
}

func TestGetValidManagedClusterSetBindings(t *testing.T) {
	placementNamespace := "ns1"
	cases := []struct {
		name                           string
		initObjs                       []runtime.Object
		expectedClusterSetBindingNames []string
	}{
		{
			name: "no bound clusterset",
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet("clusterset1"),
			},
		},
		{
			name: "invalid binding",
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSetBinding(placementNamespace, "clusterset1"),
			},
		},
		{
			name: "valid binding",
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet("clusterset1"),
				testinghelpers.NewClusterSetBinding(placementNamespace, "clusterset1"),
			},
			expectedClusterSetBindingNames: []string{"clusterset1"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)
			clusterInformerFactory := testinghelpers.NewClusterInformerFactory(clusterClient, c.initObjs...)

			ctrl := &schedulingController{
				clusterSetLister:        clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets().Lister(),
				clusterSetBindingLister: clusterInformerFactory.Cluster().V1beta1().ManagedClusterSetBindings().Lister(),
			}
			bindings, err := ctrl.getValidManagedClusterSetBindings(placementNamespace)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			expectedBindingNames := sets.NewString(c.expectedClusterSetBindingNames...)
			if len(bindings) != expectedBindingNames.Len() {
				t.Errorf("expected %d bindings but got %d", expectedBindingNames.Len(), len(bindings))
			}
			for _, binding := range bindings {
				expectedBindingNames.Delete(binding.Name)
			}
			if expectedBindingNames.Len() > 0 {
				t.Errorf("expected bindings: %s", strings.Join(expectedBindingNames.List(), ","))
			}
		})
	}
}

func TestGetAvailableClusters(t *testing.T) {
	placementNamespace := "ns1"

	cases := []struct {
		name                 string
		clusterSetNames      []string
		initObjs             []runtime.Object
		expectedClusterNames []string
	}{
		{
			name: "no clusterset",
			initObjs: []runtime.Object{
				testinghelpers.NewClusterSet("clusterset1"),
				testinghelpers.NewClusterSetBinding(placementNamespace, "clusterset1"),
			},
		},
		{
			name:            "select clusters from clustersets",
			clusterSetNames: []string{"clusterset1", "clusterset2"},
			initObjs: []runtime.Object{
				testinghelpers.NewManagedCluster("cluster1").WithLabel(clusterSetLabel, "clusterset1").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithLabel(clusterSetLabel, "clusterset2").Build(),
			},
			expectedClusterNames: []string{"cluster1", "cluster2"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)
			clusterInformerFactory := testinghelpers.NewClusterInformerFactory(clusterClient, c.initObjs...)

			ctrl := &schedulingController{
				clusterLister: clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
			}

			clusters, err := ctrl.getAvailableClusters(c.clusterSetNames)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			expectedClusterNames := sets.NewString(c.expectedClusterNames...)
			if len(clusters) != expectedClusterNames.Len() {
				t.Errorf("expected %d clusters but got %d", expectedClusterNames.Len(), len(clusters))
			}
			for _, cluster := range clusters {
				expectedClusterNames.Delete(cluster.Name)
			}
			if expectedClusterNames.Len() > 0 {
				t.Errorf("expected clusters not selected: %s", strings.Join(expectedClusterNames.List(), ","))
			}
		})
	}
}

func TestNewSatisfiedCondition(t *testing.T) {
	cases := []struct {
		name                      string
		clusterSetsInSpec         []string
		eligibleClusterSets       []string
		numOfBindings             int
		numOfAvailableClusters    int
		numOfFeasibleClusters     int
		numOfUnscheduledDecisions int
		expectedStatus            metav1.ConditionStatus
		expectedReason            string
	}{
		{
			name:                      "NoManagedClusterSetBindings",
			numOfBindings:             0,
			numOfUnscheduledDecisions: 5,
			expectedStatus:            metav1.ConditionFalse,
			expectedReason:            "NoManagedClusterSetBindings",
		},
		{
			name:                      "NoIntersection",
			clusterSetsInSpec:         []string{"clusterset1"},
			numOfBindings:             1,
			numOfAvailableClusters:    0,
			numOfFeasibleClusters:     0,
			numOfUnscheduledDecisions: 0,
			expectedStatus:            metav1.ConditionFalse,
			expectedReason:            "NoIntersection",
		},
		{
			name:                      "AllManagedClusterSetsEmpty",
			eligibleClusterSets:       []string{"clusterset1"},
			numOfBindings:             1,
			numOfAvailableClusters:    0,
			numOfFeasibleClusters:     0,
			numOfUnscheduledDecisions: 0,
			expectedStatus:            metav1.ConditionFalse,
			expectedReason:            "AllManagedClusterSetsEmpty",
		},
		{
			name:                      "NoManagedClusterMatched",
			eligibleClusterSets:       []string{"clusterset1"},
			numOfBindings:             1,
			numOfAvailableClusters:    1,
			numOfFeasibleClusters:     0,
			numOfUnscheduledDecisions: 0,
			expectedStatus:            metav1.ConditionFalse,
			expectedReason:            "NoManagedClusterMatched",
		},
		{
			name:                      "AllDecisionsScheduled",
			eligibleClusterSets:       []string{"clusterset1"},
			numOfBindings:             1,
			numOfAvailableClusters:    1,
			numOfFeasibleClusters:     1,
			numOfUnscheduledDecisions: 0,
			expectedStatus:            metav1.ConditionTrue,
			expectedReason:            "AllDecisionsScheduled",
		},
		{
			name:                      "NotAllDecisionsScheduled",
			eligibleClusterSets:       []string{"clusterset1"},
			numOfBindings:             1,
			numOfAvailableClusters:    1,
			numOfFeasibleClusters:     1,
			numOfUnscheduledDecisions: 1,
			expectedStatus:            metav1.ConditionFalse,
			expectedReason:            "NotAllDecisionsScheduled",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			condition := newSatisfiedCondition(
				c.clusterSetsInSpec,
				c.eligibleClusterSets,
				c.numOfBindings,
				c.numOfAvailableClusters,
				c.numOfFeasibleClusters,
				c.numOfUnscheduledDecisions,
				nil,
			)

			if condition.Status != c.expectedStatus {
				t.Errorf("expected status %q but got %q", c.expectedStatus, condition.Status)
			}
			if condition.Reason != c.expectedReason {
				t.Errorf("expected reason %q but got %q", c.expectedReason, condition.Reason)
			}
		})
	}
}

func TestBind(t *testing.T) {
	placementNamespace := "ns1"
	placementName := "placement1"

	cases := []struct {
		name             string
		initObjs         []runtime.Object
		clusterDecisions []clusterapiv1beta1.ClusterDecision
		validateActions  func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:             "create single placementdecision",
			clusterDecisions: newClusterDecisions(10),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "create", "update")
				actual := actions[1].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1beta1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				assertClustersSelected(t, placementDecision.Status.Decisions, newSelectedClusters(10)...)
			},
		},
		{
			name:             "create multiple placementdecisions",
			clusterDecisions: newClusterDecisions(101),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "create", "update", "create", "update")
				selectedClusters := newSelectedClusters(101)
				actual := actions[1].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1beta1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				assertClustersSelected(t, placementDecision.Status.Decisions, selectedClusters[0:100]...)

				actual = actions[3].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok = actual.(*clusterapiv1beta1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				assertClustersSelected(t, placementDecision.Status.Decisions, selectedClusters[100:]...)
			},
		},
		{
			name:             "no change",
			clusterDecisions: newClusterDecisions(128),
			initObjs: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[:100]...).Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 2)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[100:]...).Build(),
			},
			validateActions: testinghelpers.AssertNoActions,
		},
		{
			name:             "update one of placementdecisions",
			clusterDecisions: newClusterDecisions(128),
			initObjs: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[:100]...).Build(),
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "create", "update")
				selectedClusters := newSelectedClusters(128)
				actual := actions[1].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1beta1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				assertClustersSelected(t, placementDecision.Status.Decisions, selectedClusters[100:]...)
			},
		},
		{
			name:             "delete redundant placementdecisions",
			clusterDecisions: newClusterDecisions(10),
			initObjs: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[:100]...).Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 2)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[100:]...).Build(),
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update", "delete")
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1beta1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				assertClustersSelected(t, placementDecision.Status.Decisions, newSelectedClusters(10)...)
			},
		},
		{
			name:             "delete all placementdecisions",
			clusterDecisions: newClusterDecisions(0),
			initObjs: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 1)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[:100]...).Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName(placementName, 2)).
					WithLabel(placementLabel, placementName).
					WithDecisions(newSelectedClusters(128)[100:]...).Build(),
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "delete", "delete")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)

			// GenerateName is not working for fake clent, set the name with random suffix
			clusterClient.PrependReactor(
				"create",
				"placementdecisions",
				func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					createAction := action.(clienttesting.CreateActionImpl)
					pd := createAction.Object.(*clusterapiv1beta1.PlacementDecision)
					pd.Name = fmt.Sprintf("%s%s", pd.GenerateName, rand.String(5))
					return false, pd, nil
				},
			)

			clusterInformerFactory := testinghelpers.NewClusterInformerFactory(clusterClient, c.initObjs...)

			s := &testScheduler{}

			ctrl := schedulingController{
				clusterClient:           clusterClient,
				clusterLister:           clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				clusterSetLister:        clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets().Lister(),
				clusterSetBindingLister: clusterInformerFactory.Cluster().V1beta1().ManagedClusterSetBindings().Lister(),
				placementLister:         clusterInformerFactory.Cluster().V1beta1().Placements().Lister(),
				placementDecisionLister: clusterInformerFactory.Cluster().V1beta1().PlacementDecisions().Lister(),
				scheduler:               s,
				recorder:                kevents.NewFakeRecorder(100),
			}

			err := ctrl.bind(
				context.TODO(),
				testinghelpers.NewPlacement(placementNamespace, placementName).Build(),
				c.clusterDecisions,
				nil,
			)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			c.validateActions(t, clusterClient.Actions())
		})
	}
}

func assertClustersSelected(t *testing.T, decisons []clusterapiv1beta1.ClusterDecision, clusterNames ...string) {
	names := sets.NewString(clusterNames...)
	for _, decision := range decisons {
		if names.Has(decision.ClusterName) {
			names.Delete(decision.ClusterName)
		}
	}

	if names.Len() != 0 {
		t.Errorf("expected clusters selected: %s, but got %v", strings.Join(names.UnsortedList(), ","), decisons)
	}
}

func newClusterDecisions(num int) (decisions []clusterapiv1beta1.ClusterDecision) {
	for i := 0; i < num; i++ {
		decisions = append(decisions, clusterapiv1beta1.ClusterDecision{
			ClusterName: fmt.Sprintf("cluster%d", i+1),
		})
	}
	return decisions
}

func newSelectedClusters(num int) (clusters []string) {
	for i := 0; i < num; i++ {
		clusters = append(clusters, fmt.Sprintf("cluster%d", i+1))
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		return clusters[i] < clusters[j]
	})

	return clusters
}
