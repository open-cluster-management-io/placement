package scalability

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	clusterapiv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

var prioritizerPolicy = &clusterapiv1beta1.PrioritizerPolicy{
	Mode: clusterapiv1beta1.PrioritizerPolicyModeExact,
	Configurations: []clusterapiv1beta1.PrioritizerConfig{
		{
			ScoreCoordinate: &clusterapiv1beta1.ScoreCoordinate{
				Type: clusterapiv1beta1.ScoreCoordinateTypeAddOn,
				AddOn: &clusterapiv1beta1.AddOnScore{
					ResourceName: "demo",
					ScoreName:    "demo",
				},
			},
			Weight: 1,
		},
	},
}

var _ = ginkgo.Describe("Placement addon score scalability test", func() {
	var namespace string
	var placementSetName string
	var clusterSetName string
	var suffix string
	var err error

	ginkgo.BeforeEach(func() {
		suffix = rand.String(5)
		namespace = fmt.Sprintf("ns-%s", suffix)
		placementSetName = fmt.Sprintf("placementset-%s", suffix)
		clusterSetName = fmt.Sprintf("clusterset-%s", suffix)

		// create testing namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		assertBindingClusterSet(namespace, clusterSetName)
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("Delete placement")
		err = clusterClient.ClusterV1beta1().Placements(namespace).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: placementSetLabel + "=" + placementSetName,
		})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		ginkgo.By("Delete managedclusterset")
		clusterClient.ClusterV1beta2().ManagedClusterSets().Delete(context.Background(), clusterSetName, metav1.DeleteOptions{})

		ginkgo.By("Delete managedclusters")
		clusterClient.ClusterV1().ManagedClusters().DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: clusterSetLabel + "=" + clusterSetName,
		})

		ginkgo.By("Delete addonscores and namespaces")
		scores, _ := clusterClient.ClusterV1alpha1().AddOnPlacementScores("").List(context.Background(), metav1.ListOptions{})
		for _, s := range scores.Items {
			err = clusterClient.ClusterV1alpha1().AddOnPlacementScores(s.Namespace).Delete(context.Background(), s.Name, metav1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			err = kubeClient.CoreV1().Namespaces().Delete(context.Background(), s.Namespace, metav1.DeleteOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		}

		err = kubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	})

	/* To check the scalability of placement addon score updating, we will
	 * 1. create N managedclusters, and create M placements whose NumberOfClusters is N-1, and create N-1 AddOnPlacementScores with score range 0-99.
	 * Then select several placements to ensure each one has N-1 decisions, this is for creating.
	 * 2. create the Nth AddOnPlacementScores for Nth cluster with score 100
	 * Then check placements to ensure each one has cluster N in decisions, this is for updating.
	 * M will be 10, 100, 300
	 * N will be 100
	 */

	ginkgo.It("Should update placement efficiently when addon score changes", func() {
		totalPlacements := 10
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create %v placement with %v managedclusters", totalPlacements, totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		clusterNames := assertCreatingClusters(totalClusters, clusterSetName)

		assertCreatingAddOnPlacementScores("demo", "demo", clusterNames[:len(clusterNames)-1])

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("createtime", func() {
				assertCreatingPlacements(totalPlacements, noc(totalClusters-1), totalClusters-1, prioritizerPolicy, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("createtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 50), "Something during creating placement take too long.")

		//update cluster N-1 and cluster N score as 100
		ginkgo.By("Update AddOnPlacementScores for 2 clusters")
		assertCreatingAddOnPlacementScore("demo", "demo", clusterNames[len(clusterNames)-2], 100)
		assertCreatingAddOnPlacementScore("demo", "demo", clusterNames[len(clusterNames)-1], 100)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("updatetime", func() {
				assertUpdatingClusterOfDecisions(totalPlacements, clusterNames[len(clusterNames)-1], namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats = experiment.GetStats("updatetime")
		medianDuration = repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 5), "Something during updating placement take too long.")
	})

	ginkgo.It("Should update placement efficiently when addon score changes", func() {
		totalPlacements := 100
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create %v placement with %v managedclusters", totalPlacements, totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		clusterNames := assertCreatingClusters(totalClusters, clusterSetName)

		assertCreatingAddOnPlacementScores("demo", "demo", clusterNames[:len(clusterNames)-1])

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("createtime", func() {
				assertCreatingPlacements(totalPlacements, noc(totalClusters-1), totalClusters-1, prioritizerPolicy, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("createtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 500), "Something during creating placement take too long.")

		//update cluster N-1 and cluster N score as 100
		ginkgo.By("Update AddOnPlacementScores for 2 clusters")
		assertCreatingAddOnPlacementScore("demo", "demo", clusterNames[len(clusterNames)-2], 100)
		assertCreatingAddOnPlacementScore("demo", "demo", clusterNames[len(clusterNames)-1], 100)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("updatetime", func() {
				assertUpdatingClusterOfDecisions(totalPlacements, clusterNames[len(clusterNames)-1], namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats = experiment.GetStats("updatetime")
		medianDuration = repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 40), "Something during updating placement take too long.")
	})

	ginkgo.It("Should update placement efficiently when addon score changes", func() {
		totalPlacements := 300
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create %v placement with %v managedclusters", totalPlacements, totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		clusterNames := assertCreatingClusters(totalClusters, clusterSetName)

		assertCreatingAddOnPlacementScores("demo", "demo", clusterNames[:len(clusterNames)-1])

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("createtime", func() {
				assertCreatingPlacements(totalPlacements, noc(totalClusters-1), totalClusters-1, prioritizerPolicy, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("createtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 1500), "Something during creating placement take too long.")

		//update cluster N-1 and cluster N score as 100
		ginkgo.By("Update AddOnPlacementScores for 2 clusters")
		assertCreatingAddOnPlacementScore("demo", "demo", clusterNames[len(clusterNames)-2], 100)
		assertCreatingAddOnPlacementScore("demo", "demo", clusterNames[len(clusterNames)-1], 100)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("updatetime", func() {
				assertUpdatingClusterOfDecisions(totalPlacements, clusterNames[len(clusterNames)-1], namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats = experiment.GetStats("updatetime")
		medianDuration = repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 120), "Something during updating placement take too long.")
	})
})
