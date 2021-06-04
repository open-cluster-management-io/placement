package plugins

import (
	"testing"

	clusterfake "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	clusterapiv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	testinghelpers "github.com/open-cluster-management/placement/pkg/helpers/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestSchedule(t *testing.T) {
	placementNamespace := "ns1"
	placementName := "placement1"
	placementDecisionName := "placement1-decision1"
	cases := []struct {
		name             string
		placement        *clusterapiv1alpha1.Placement
		initObjs         []runtime.Object
		clusters         []*clusterapiv1.ManagedCluster
		expectedClusters sets.String
	}{
		{
			name:      "Scheduled cluster",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(2).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, placementDecisionName).
					WithLabel(placementLabel, placementName).
					WithDecisions("cluster2", "cluster3").Build(),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").Build(),
				testinghelpers.NewManagedCluster("cluster2").Build(),
				testinghelpers.NewManagedCluster("cluster3").Build(),
			},
			expectedClusters: sets.NewString("cluster2", "cluster3"),
		},
		{
			name:      "Balance",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).WithNOC(2).Build(),
			initObjs: []runtime.Object{
				testinghelpers.NewPlacementDecision(placementNamespace, "decision2").
					WithLabel(placementLabel, "placement2").
					WithDecisions("cluster1", "cluster2").Build(),
				testinghelpers.NewPlacementDecision(placementNamespace, "decision3").
					WithLabel(placementLabel, "placement3").
					WithDecisions("cluster1", "cluster3").Build(),
			},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").Build(),
				testinghelpers.NewManagedCluster("cluster2").Build(),
				testinghelpers.NewManagedCluster("cluster3").Build(),
			},
			expectedClusters: sets.NewString("cluster2", "cluster3"),
		},
		{
			name: "Hard Affinity",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).
				AddAffinity(
					nil,
					&clusterapiv1alpha1.ClusterClaimSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "cloud",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"Amazon"},
							},
						},
					},
					0,
				).WithNOC(2).Build(),
			initObjs: []runtime.Object{},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithClaim("cloud", "Amazon").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithClaim("cloud", "Google").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithClaim("cloud", "Google").Build(),
			},
			expectedClusters: sets.NewString("cluster1"),
		},
		{
			name: "Hard Spread",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).
				AddSpread("Claim", "cloud", 1).WithNOC(3).Build(),
			initObjs: []runtime.Object{},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithClaim("cloud", "Amazon").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithClaim("cloud", "Amazon").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithClaim("cloud", "Google").Build(),
				testinghelpers.NewManagedCluster("cluster4").WithClaim("cloud", "Google").Build(),
				testinghelpers.NewManagedCluster("cluster5").WithClaim("cloud", "IBM").Build(),
			},
			expectedClusters: sets.NewString("cluster1", "cluster3", "cluster5"),
		},
		{
			name: "Soft Affinity",
			placement: testinghelpers.NewPlacement(placementNamespace, placementName).
				AddAffinity(
					nil,
					&clusterapiv1alpha1.ClusterClaimSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "cloud",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"Amazon"},
							},
						},
					},
					1,
				).
				AddAffinity(
					&metav1.LabelSelector{
						MatchLabels: map[string]string{
							"vendor": "OpenShift",
						},
					},
					nil,
					1,
				).
				WithNOC(2).Build(),
			initObjs: []runtime.Object{},
			clusters: []*clusterapiv1.ManagedCluster{
				testinghelpers.NewManagedCluster("cluster1").WithClaim("cloud", "Amazon").WithLabel("vendor", "OpenShift").Build(),
				testinghelpers.NewManagedCluster("cluster2").WithClaim("cloud", "Google").Build(),
				testinghelpers.NewManagedCluster("cluster3").WithClaim("cloud", "Google").WithLabel("vendor", "OpenShift").Build(),
			},
			expectedClusters: sets.NewString("cluster1", "cluster3"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.initObjs = append(c.initObjs, c.placement)
			clusterClient := clusterfake.NewSimpleClientset(c.initObjs...)
			clusterInformerFactory := testinghelpers.NewClusterInformerFactory(clusterClient, c.initObjs...)
			scheduler := NewScheduler(clusterInformerFactory.Cluster().V1alpha1().PlacementDecisions().Lister())
			selected := scheduler.Schedule(c.placement, c.clusters)
			if !selected.Equal(c.expectedClusters) {
				t.Errorf("unexpected selected clusters, expected %v, actual %v", c.expectedClusters, selected)
			}
		})
	}
}
