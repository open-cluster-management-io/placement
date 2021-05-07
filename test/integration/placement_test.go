package integration

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	controllers "github.com/open-cluster-management/placement/pkg/controllers"
	"github.com/open-cluster-management/placement/test/integration/util"
)

const (
	placementLabel = "cluster.open-cluster-management.io/placement"
)

var _ = ginkgo.Describe("Placement Scheduling", func() {
	var cancel context.CancelFunc
	var namespace string
	var placementName string
	var suffix string
	var err error

	ginkgo.BeforeEach(func() {
		suffix = rand.String(5)
		namespace = fmt.Sprintf("ns-%s", suffix)
		placementName = fmt.Sprintf("placement-%s", suffix)

		// create testing namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// start controller manager
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		go controllers.RunControllerManager(ctx, &controllercmd.ControllerContext{
			KubeConfig:    restConfig,
			EventRecorder: util.NewIntegrationTestEventRecorder("integration"),
		})
	})

	ginkgo.AfterEach(func() {
		if cancel != nil {
			cancel()
		}
		err := kubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	})

	assertPlacementDecisionCreated := func(placement *clusterapiv1alpha1.Placement) {
		ginkgo.By("Check if placementdecision is created")
		gomega.Eventually(func() bool {
			pdl, err := clusterClient.ClusterV1alpha1().PlacementDecisions(namespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: placementLabel + "=" + placement.Name,
			})
			if err != nil {
				return false
			}
			if len(pdl.Items) == 0 {
				return false
			}
			for _, pd := range pdl.Items {
				if controlled := metav1.IsControlledBy(&pd.ObjectMeta, placement); !controlled {
					return false
				}
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	}

	assertPlacementDeleted := func(placementName string) {
		ginkgo.By("Check if placement is gone")
		gomega.Eventually(func() bool {
			_, err := clusterClient.ClusterV1alpha1().Placements(namespace).Get(context.Background(), placementName, metav1.GetOptions{})
			if err == nil {
				return false
			}
			return errors.IsNotFound(err)
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	}

	assertNumberOfDecisions := func(placementName string, desiredNOD int) {
		ginkgo.By("Check the number of decisions in placementdecisions")
		gomega.Eventually(func() bool {
			pdl, err := clusterClient.ClusterV1alpha1().PlacementDecisions(namespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: placementLabel + "=" + placementName,
			})
			if err != nil {
				return false
			}
			if len(pdl.Items) == 0 {
				return false
			}
			actualNOD := 0
			for _, pd := range pdl.Items {
				actualNOD += len(pd.Status.Decisions)
			}
			return actualNOD == desiredNOD
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	}

	assertPlacementStatus := func(placementName string, numOfSelectedClusters int32, satisfied bool) {
		ginkgo.By("Check the status of placement")
		gomega.Eventually(func() bool {
			placement, err := clusterClient.ClusterV1alpha1().Placements(namespace).Get(context.Background(), placementName, metav1.GetOptions{})
			if err != nil {
				return false
			}

			if satisfied && !util.HasCondition(
				placement.Status.Conditions,
				clusterapiv1alpha1.PlacementConditionSatisfied,
				"AllDecisionsScheduled",
				metav1.ConditionTrue,
			) {
				return false
			}

			if !satisfied && !util.HasCondition(
				placement.Status.Conditions,
				clusterapiv1alpha1.PlacementConditionSatisfied,
				"NotAllDecisionsScheduled",
				metav1.ConditionFalse,
			) {
				return false
			}

			return placement.Status.NumberOfSelectedClusters == numOfSelectedClusters
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	}

	ginkgo.It("Should create placement with NOC successfully", func() {
		ginkgo.By("Create placement")
		num := int32(10)
		placement := &clusterapiv1alpha1.Placement{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      placementName,
			},
			Spec: clusterapiv1alpha1.PlacementSpec{
				NumberOfClusters: &num,
			},
		}
		placement, err = clusterClient.ClusterV1alpha1().Placements(namespace).Create(context.Background(), placement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		assertPlacementDecisionCreated(placement)
		assertPlacementStatus(placementName, 0, false)

		ginkgo.By("Delete placementdecisions")
		pdl, err := clusterClient.ClusterV1alpha1().PlacementDecisions(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: placementLabel + "=" + placementName,
		})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		for _, pd := range pdl.Items {
			err = clusterClient.ClusterV1alpha1().PlacementDecisions(namespace).Delete(context.Background(), pd.Name, metav1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		}
		assertPlacementDecisionCreated(placement)
		assertNumberOfDecisions(placementName, int(num))
		assertPlacementStatus(placementName, 0, false)

		ginkgo.By("Increase NOC of the placement")
		placement, err = clusterClient.ClusterV1alpha1().Placements(namespace).Get(context.Background(), placementName, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		num = int32(20)
		placement.Spec.NumberOfClusters = &num
		placement, err = clusterClient.ClusterV1alpha1().Placements(namespace).Update(context.Background(), placement, metav1.UpdateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		assertNumberOfDecisions(placementName, int(num))
		assertPlacementStatus(placementName, 0, false)

		ginkgo.By("Reduce NOC of the placement")
		placement, err = clusterClient.ClusterV1alpha1().Placements(namespace).Get(context.Background(), placementName, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		num = int32(5)
		placement.Spec.NumberOfClusters = &num
		placement, err = clusterClient.ClusterV1alpha1().Placements(namespace).Update(context.Background(), placement, metav1.UpdateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		assertNumberOfDecisions(placementName, int(num))
		assertPlacementStatus(placementName, 0, false)

		ginkgo.By("Delete placement")
		err = clusterClient.ClusterV1alpha1().Placements(namespace).Delete(context.Background(), placementName, metav1.DeleteOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		assertPlacementDeleted(placementName)
	})
})
