package scalability

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	clusterapiv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	"open-cluster-management.io/placement/test/integration/util"
)

var sampleCount = 10

func assertCreatingNamespace(name string) {
	var err error
	if _, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func assertBindingClusterSet(namespace, clusterSetName string) {
	ginkgo.By("Create clusterset/clustersetbinding")
	clusterset := &clusterapiv1beta2.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterSetName,
		},
	}
	_, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), clusterset, metav1.CreateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	csb := &clusterapiv1beta2.ManagedClusterSetBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterSetName,
		},
		Spec: clusterapiv1beta2.ManagedClusterSetBindingSpec{
			ClusterSet: clusterSetName,
		},
	}
	_, err = clusterClient.ClusterV1beta2().ManagedClusterSetBindings(namespace).Create(context.Background(), csb, metav1.CreateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func assertCreatingClusters(num int, clusterSetName string) []string {
	var err error
	clusterNames := []string{}
	ginkgo.By(fmt.Sprintf("Create %d clusters", num))
	for i := 0; i < num; i++ {
		cluster := &clusterapiv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "cluster-",
				Labels: map[string]string{
					clusterSetLabel: clusterSetName,
				},
			},
		}
		cluster, err = clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), cluster, metav1.CreateOptions{})
		clusterNames = append(clusterNames, cluster.Name)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	return clusterNames
}

func assertCreatingPlacements(num int, noc *int32, nod int, prioritizerPolicy *clusterapiv1beta1.PrioritizerPolicy, namespace, placementSetName string) {
	ginkgo.By(fmt.Sprintf("Create %d placements", num))
	for i := 0; i < num; i++ {
		assertCreatingPlacement(noc, nod, prioritizerPolicy, namespace, placementSetName)
		//		time.Sleep(time.Duration(1) * time.Second) //sleep 1 second in case API server is too busy
	}
}

func assertCreatingPlacement(noc *int32, nod int, prioritizerPolicy *clusterapiv1beta1.PrioritizerPolicy, namespace, placementSetName string) {
	placement := &clusterapiv1beta1.Placement{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "placement-",
			Labels: map[string]string{
				placementSetLabel: placementSetName,
			},
		},
		Spec: clusterapiv1beta1.PlacementSpec{
			NumberOfClusters: noc,
		},
	}
	if prioritizerPolicy != nil {
		placement.Spec.PrioritizerPolicy = *prioritizerPolicy
	}
	pl, err := clusterClient.ClusterV1beta1().Placements(namespace).Create(context.Background(), placement, metav1.CreateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = assertNumberOfDecisions(pl, nod)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	if noc != nil {
		err = assertPlacementStatus(pl, nod, nod == int(*noc))
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func assertPlacementStatus(placement *clusterapiv1beta1.Placement, numOfSelectedClusters int, satisfied bool) error {
	var localerr error
	gomega.Eventually(func() bool {
		localerr = nil
		placement, err := clusterClient.ClusterV1beta1().Placements(placement.Namespace).Get(context.Background(), placement.Name, metav1.GetOptions{})
		if err != nil {
			localerr = err
			return false
		}
		status := metav1.ConditionFalse
		if satisfied {
			status = metav1.ConditionTrue
		}
		if !util.HasCondition(
			placement.Status.Conditions,
			clusterapiv1beta1.PlacementConditionSatisfied,
			"",
			status,
		) {
			localerr = errors.New("Contition check failed")
			return false
		}
		if placement.Status.NumberOfSelectedClusters != int32(numOfSelectedClusters) {
			localerr = errors.New(fmt.Sprintf("Mismatch value %v:%v", placement.Status.NumberOfSelectedClusters, int32(numOfSelectedClusters)))
		}
		return placement.Status.NumberOfSelectedClusters == int32(numOfSelectedClusters)
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

	return localerr
}

func assertUpdatingNumberOfDecisions(num int, nod int, namespace, placementSetName string) {
	ginkgo.By(fmt.Sprintf("Check %v updated placements", num))

	pls, err := clusterClient.ClusterV1beta1().Placements(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: placementSetLabel + "=" + placementSetName,
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	sampleCap := num / sampleCount
	targetSample := 0
	currentSample := 0

	for _, pl := range pls.Items {
		currentSample++
		if currentSample > targetSample {
			targetSample += sampleCap
			err := assertNumberOfDecisions(&pl, nod)
			if err != nil {
				break
			}
		}
	}

	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func assertNumberOfDecisions(placement *clusterapiv1beta1.Placement, desiredNOD int) error {
	var localerr error
	gomega.Eventually(func() bool {
		localerr = nil
		pdl, err := clusterClient.ClusterV1beta1().PlacementDecisions(placement.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: placementLabel + "=" + placement.Name,
		})
		if err != nil {
			localerr = err
			return false
		}
		if len(pdl.Items) == 0 {
			localerr = errors.New("No placementdecision found")
			return false
		}

		actualNOD := 0
		for _, pd := range pdl.Items {
			if controlled := metav1.IsControlledBy(&pd.ObjectMeta, placement); !controlled {
				localerr = errors.New("No controllerRef found for a placement")
				return false
			}
			actualNOD += len(pd.Status.Decisions)
		}
		if actualNOD != desiredNOD {
			localerr = errors.New(fmt.Sprintf("Mismatch value %v:%v", actualNOD, desiredNOD))
		}
		return actualNOD == desiredNOD
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

	return localerr
}

func assertUpdatingClusterOfDecisions(num int, targetCluster string, namespace, placementSetName string) {
	ginkgo.By(fmt.Sprintf("Check %v updated placements", num))

	pls, err := clusterClient.ClusterV1beta1().Placements(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: placementSetLabel + "=" + placementSetName,
	})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	for _, pl := range pls.Items {
		err = assertClusterOfDecisions(&pl, targetCluster)
		if err != nil {
			break
		}
	}
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func assertClusterOfDecisions(placement *clusterapiv1beta1.Placement, targetCluster string) error {
	var localerr error
	gomega.Eventually(func() bool {
		localerr = nil
		pdl, err := clusterClient.ClusterV1beta1().PlacementDecisions(placement.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: placementLabel + "=" + placement.Name,
		})
		if err != nil {
			localerr = err
			return false
		}
		if len(pdl.Items) == 0 {
			localerr = errors.New("No placementdecision found")
			return false
		}

		for _, pd := range pdl.Items {
			if controlled := metav1.IsControlledBy(&pd.ObjectMeta, placement); !controlled {
				localerr = errors.New("No controllerRef found for a placement")
				return false
			}
			for _, c := range pd.Status.Decisions {
				if c.ClusterName == targetCluster {
					return true
				}
			}
		}
		return false
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

	return localerr
}

func assertCreatingAddOnPlacementScores(crname, scorename string, clusterNames []string) {
	ginkgo.By(fmt.Sprintf("Create %v AddOnPlacementScores", len(clusterNames)))

	for _, name := range clusterNames {
		assertCreatingAddOnPlacementScore(crname, scorename, name, rand.Int31n(99))
	}
}

func assertCreatingAddOnPlacementScore(crname, scorename, clusternamespace string, score int32) {
	assertCreatingNamespace(clusternamespace)

	addOn, err := clusterClient.ClusterV1alpha1().AddOnPlacementScores(clusternamespace).Get(context.Background(), crname, metav1.GetOptions{})
	if err != nil {
		newAddOn := &clusterapiv1alpha1.AddOnPlacementScore{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: clusternamespace,
				Name:      crname,
			},
		}
		addOn, err = clusterClient.ClusterV1alpha1().AddOnPlacementScores(clusternamespace).Create(context.Background(), newAddOn, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	addOn.Status = clusterapiv1alpha1.AddOnPlacementScoreStatus{
		Scores: []clusterapiv1alpha1.AddOnPlacementScoreItem{
			{
				Name:  scorename,
				Value: score,
			},
		},
	}

	_, err = clusterClient.ClusterV1alpha1().AddOnPlacementScores(clusternamespace).UpdateStatus(context.Background(), addOn, metav1.UpdateOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}
