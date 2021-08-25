package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	clusterv1client "open-cluster-management.io/api/client/cluster/clientset/versioned"
)

const (
	eventuallyTimeout  = 30 // seconds
	eventuallyInterval = 1  // seconds
)

var testEnv *envtest.Environment
var restConfig *rest.Config
var kubeClient kubernetes.Interface
var clusterClient clusterv1client.Interface
var enableTestEnv bool = true

func TestIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Integration Suite")
}

var _ = ginkgo.BeforeSuite(func(done ginkgo.Done) {
	// If KUBE_TEST_ENV is false, will connect to a real kube env defined in KUBECONFIG.
	if os.Getenv("KUBE_TEST_ENV") == "false" {
		enableTestEnv = false
	}

	if enableTestEnv {
		initKubeTestEnv()
	} else {
		initKubeRealEnv()
	}
	close(done)
}, 60)

var _ = ginkgo.AfterSuite(func() {
	if enableTestEnv {
		ginkgo.By("tearing down the test environment")

		err := testEnv.Stop()
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
})

func initKubeRealEnv() {
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")

	// setup kubeclient and clusterclient
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	clusterClient, err = clusterv1client.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func initKubeTestEnv() {
	ginkgo.By("bootstrapping test environment")

	// start a kube-apiserver
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "cluster", "v1"),
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "cluster", "v1alpha1"),
		},
	}

	cfg, err := testEnv.Start()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(cfg).ToNot(gomega.BeNil())

	// setup kubeclient and clusterclient
	restConfig = cfg
	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	clusterClient, err = clusterv1client.NewForConfig(restConfig)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

}
