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
)

const (
	clusterSetLabel   = "cluster.open-cluster-management.io/clusterset"
	placementLabel    = "cluster.open-cluster-management.io/placement"
	placementSetLabel = "cluster.open-cluster-management.io/placementset"
)

var _ = ginkgo.Describe("Placement scalability test", func() {
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

		err = kubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	})

	/* we create N managedclusters here, and create a placement whose NumberOfClusters is N-1 to ensure the placement logic will
	 * do comparison to select N-1 managedclusters from N candidates
	 * N will be 100, 1000, 2000.
	 */

	ginkgo.It("Should create placement efficiently", func() {
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create 1 placement with %v managedclusters", totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		assertCreatingClusters(totalClusters, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("runtime", func() {
				assertCreatingPlacement(noc(totalClusters-1), totalClusters-1, nil, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 5, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("runtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)

		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 5), "Something during creating placement take too long.")
	})

	ginkgo.It("Should create placement efficiently", func() {
		totalClusters := 1000

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create 1 placement with %v managedclusters", totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		assertCreatingClusters(totalClusters, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("runtime", func() {
				assertCreatingPlacement(noc(totalClusters-1), totalClusters-1, nil, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 5, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("runtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)

		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 5), "Something during creating placement take too long.")
	})

	ginkgo.It("Should create placement efficiently", func() {
		totalClusters := 2000

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create 1 placement with %v managedclusters", totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		assertCreatingClusters(totalClusters, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("runtime", func() {
				assertCreatingPlacement(noc(totalClusters-1), totalClusters-1, nil, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 5, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("runtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)

		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 10), "Something during creating placement take too long.")
	})

	/* To check the scalability of placement creating/updating, we will
	 * 1. create N-2 managedclusters, and create M placements whose NumberOfClusters is N-1, then select several placements to ensure each one has N-2 decisions, this is for creating.
	 * 2. create 2 managedclusters, then select several placements to ensure each one has N-1 decisions, this is for updating.
	 * M will be 10, 100, 300
	 * N will be 100
	 */

	ginkgo.It("Should create/update placement efficiently", func() {
		totalPlacements := 10
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create %v placement with %v managedclusters", totalPlacements, totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		assertCreatingClusters(totalClusters-2, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("createtime", func() {
				assertCreatingPlacements(totalPlacements, noc(totalClusters-1), totalClusters-2, nil, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("createtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 50), "Something during creating placement take too long.")

		assertCreatingClusters(2, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("updatetime", func() {
				assertUpdatingNumberOfDecisions(totalPlacements, totalClusters-1, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats = experiment.GetStats("updatetime")
		medianDuration = repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 5), "Something during updating placement take too long.")
	})

	ginkgo.It("Should create/update placement efficiently", func() {
		totalPlacements := 100
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create %v placement with %v managedclusters", totalPlacements, totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		assertCreatingClusters(totalClusters-2, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("createtime", func() {
				assertCreatingPlacements(totalPlacements, noc(totalClusters-1), totalClusters-2, nil, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("createtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 500), "Something during creating placement take too long.")

		assertCreatingClusters(2, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("updatetime", func() {
				assertUpdatingNumberOfDecisions(totalPlacements, totalClusters-1, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats = experiment.GetStats("updatetime")
		medianDuration = repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 40), "Something during updating placement take too long.")
	})

	ginkgo.It("Should create/update placement efficiently", func() {
		totalPlacements := 300
		totalClusters := 100

		experiment := gmeasure.NewExperiment(fmt.Sprintf("Create %v placement with %v managedclusters", totalPlacements, totalClusters))
		ginkgo.AddReportEntry(experiment.Name, experiment)

		assertCreatingClusters(totalClusters-2, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("createtime", func() {
				assertCreatingPlacements(totalPlacements, noc(totalClusters-1), totalClusters-2, nil, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats := experiment.GetStats("createtime")
		medianDuration := repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 1500), "Something during creating placement take too long.")

		assertCreatingClusters(2, clusterSetName)

		experiment.Sample(func(idx int) {
			experiment.MeasureDuration("updatetime", func() {
				assertUpdatingNumberOfDecisions(totalPlacements, totalClusters-1, namespace, placementSetName)
			})
		}, gmeasure.SamplingConfig{N: 1, Duration: time.Minute})

		//get the median repagination duration from the experiment we just ran
		repaginationStats = experiment.GetStats("updatetime")
		medianDuration = repaginationStats.DurationFor(gmeasure.StatMedian)
		gomega.Ω(medianDuration.Seconds()).Should(gomega.BeNumerically("<", 120), "Something during updating placement take too long.")
	})
})

func noc(n int) *int32 {
	noc := int32(n)
	return &noc
}
