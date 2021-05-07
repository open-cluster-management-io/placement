package scheduling

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clienttesting "k8s.io/client-go/testing"

	clusterfake "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	clusterinformers "github.com/open-cluster-management/api/client/cluster/informers/externalversions"
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	testinghelpers "github.com/open-cluster-management/placement/pkg/helpers/testing"
)

func TestDecisionPlaceholderControllerSync(t *testing.T) {
	placementNamespace := "ns1"
	placementName := "placement1"
	queueKey := placementNamespace + "/" + placementName

	cases := []struct {
		name                   string
		queueKey               string
		initObjs               []runtime.Object
		expectedNumOfDecisions int
		validateActions        func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:            "placement not found",
			queueKey:        queueKey,
			validateActions: testinghelpers.AssertNoActions,
		},
		{
			name:     "placement is deleting",
			queueKey: queueKey,
			initObjs: []runtime.Object{
				testinghelpers.NewPlacement(placementNamespace, placementName).WithDeletionTimestamp().Build(),
			},
			validateActions: testinghelpers.AssertNoActions,
		},
		{
			name:     "placement with noc",
			queueKey: queueKey,
			initObjs: []runtime.Object{
				testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(10).Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, "decision1").WithPlacementLabel(placementName).Build(),
			},
			expectedNumOfDecisions: 10,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update", "update")
				// check placementdecision
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1alpha1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				if len(placementDecision.Status.Decisions) != 10 {
					t.Errorf("expected 10 decisions created, but got %d", len(placementDecision.Status.Decisions))
				}
				// check placement status
				actual = actions[1].(clienttesting.UpdateActionImpl).Object
				placement, ok := actual.(*clusterapiv1alpha1.Placement)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				if !testinghelpers.HasCondition(placement.Status.Conditions,
					clusterapiv1alpha1.PlacementConditionSatisfied, "NotAllDecisionsScheduled", metav1.ConditionFalse) {
					t.Errorf("expect placement unsatisfied")
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)
			clusterInformerFactory := clusterinformers.NewSharedInformerFactory(clusterClient, time.Minute*10)
			clusterStore := clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().GetStore()
			clusterSetBindingStore := clusterInformerFactory.Cluster().V1alpha1().ManagedClusterSetBindings().Informer().GetStore()
			placementStore := clusterInformerFactory.Cluster().V1alpha1().Placements().Informer().GetStore()
			placementDecisionStore := clusterInformerFactory.Cluster().V1alpha1().PlacementDecisions().Informer().GetStore()
			for _, obj := range c.initObjs {
				switch obj.(type) {
				case *clusterapiv1.ManagedCluster:
					clusterStore.Add(obj)
				case *clusterapiv1alpha1.ManagedClusterSetBinding:
					clusterSetBindingStore.Add(obj)
				case *clusterapiv1alpha1.Placement:
					placementStore.Add(obj)
				case *clusterapiv1alpha1.PlacementDecision:
					placementDecisionStore.Add(obj)
				}
			}

			ctrl := decisionPlaceholderController{
				clusterClient:           clusterClient,
				clusterLister:           clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				placementLister:         clusterInformerFactory.Cluster().V1alpha1().Placements().Lister(),
				placementDecisionLister: clusterInformerFactory.Cluster().V1alpha1().PlacementDecisions().Lister(),
			}
			syncErr := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.queueKey))
			if syncErr != nil {
				t.Errorf("unexpected err: %v", syncErr)
			}

			if c.validateActions != nil {
				c.validateActions(t, clusterClient.Actions())
			}
		})
	}
}

func TestUpdateClusterDecisions(t *testing.T) {
	placementNamespace := "ns1"
	placementDecisionName := "decision1"
	clusterNames := []string{}
	for i := 0; i < 10; i++ {
		clusterNames = append(clusterNames, fmt.Sprintf("cluster-%d", i))
	}

	cases := []struct {
		name                    string
		placementDecision       *clusterapiv1alpha1.PlacementDecision
		numOfDecisions          int
		numOfScheduledDecisions int
		validateActions         func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:              "placementdecision without decisions",
			placementDecision: testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName).Build(),
			numOfDecisions:    10,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1alpha1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				if len(placementDecision.Status.Decisions) != 10 {
					t.Errorf("expected 10 decisions created, but got %d", len(placementDecision.Status.Decisions))
				}
			},
		},
		{
			name:                    "placementdecision with few decisions",
			placementDecision:       testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName).WithDecisions(newDecisions(clusterNames[:5]...)).Build(),
			numOfDecisions:          10,
			numOfScheduledDecisions: 5,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1alpha1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				if len(placementDecision.Status.Decisions) != 10 {
					t.Errorf("expected 10 decisions created, but got %d", len(placementDecision.Status.Decisions))
				}

				assertClustersSelected(t, placementDecision.Status.Decisions, clusterNames[:5]...)
			},
		},
		{
			name:              "placementdecision with desired number of decisions",
			placementDecision: testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName).WithDecisions(newPlaceholders(10)).Build(),
			numOfDecisions:    10,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected PlacementDecision was not updated")
				}
			},
		},
		{
			name: "remove empty decisions",
			placementDecision: testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName).WithDecisions(
				append(newPlaceholders(8), newDecisions(clusterNames[:5]...)...)).Build(),
			numOfDecisions:          10,
			numOfScheduledDecisions: 5,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1alpha1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				if len(placementDecision.Status.Decisions) != 10 {
					t.Errorf("expected 10 decisions created, but got %d", len(placementDecision.Status.Decisions))
				}

				assertClustersSelected(t, placementDecision.Status.Decisions, clusterNames[:5]...)
			},
		},
		{
			name:                    "decision slice truncated",
			placementDecision:       testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName).WithDecisions(newDecisions(clusterNames...)).Build(),
			numOfDecisions:          5,
			numOfScheduledDecisions: 5,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				actual := actions[0].(clienttesting.UpdateActionImpl).Object
				placementDecision, ok := actual.(*clusterapiv1alpha1.PlacementDecision)
				if !ok {
					t.Errorf("expected PlacementDecision was updated")
				}
				if len(placementDecision.Status.Decisions) != 5 {
					t.Errorf("expected 10 decisions created, but got %d", len(placementDecision.Status.Decisions))
				}

				assertClustersSelected(t, placementDecision.Status.Decisions, clusterNames[:5]...)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := clusterfake.NewSimpleClientset(c.placementDecision)
			numOfScheduledDecisions, err := updateClusterDecisions(
				context.TODO(),
				[]*clusterapiv1alpha1.PlacementDecision{c.placementDecision},
				c.numOfDecisions,
				clusterClient)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			if numOfScheduledDecisions != c.numOfScheduledDecisions {
				t.Errorf("expecte %d cluster decisions scheduled, but got %d", c.numOfScheduledDecisions, numOfScheduledDecisions)
			}

			c.validateActions(t, clusterClient.Actions())
		})
	}
}

func newDecisions(clusterNames ...string) (decisons []clusterapiv1alpha1.ClusterDecision) {
	for _, clusterName := range clusterNames {
		decisons = append(decisons, clusterapiv1alpha1.ClusterDecision{
			ClusterName: clusterName,
		})
	}
	return decisons
}

func assertClustersSelected(t *testing.T, decisons []clusterapiv1alpha1.ClusterDecision, clusterNames ...string) {
	names := sets.NewString(clusterNames...)
	for _, decision := range decisons {
		if names.Has(decision.ClusterName) {
			names.Delete(decision.ClusterName)
		}
	}

	if names.Len() != 0 {
		t.Errorf("expected clusters selected: %s", strings.Join(names.UnsortedList(), ","))
	}
}
