package benchmark

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	clusterv1client "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	controllers "open-cluster-management.io/placement/pkg/controllers"
	"open-cluster-management.io/placement/test/integration/util"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var cfg *rest.Config
var kubeClient kubernetes.Interface
var clusterClient clusterv1client.Interface

var namespace, name = "benchmark", "benchmark"
var noc1 = int32(1)

var benchmarkPlacement = clusterapiv1alpha1.Placement{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	},
	Spec: clusterapiv1alpha1.PlacementSpec{
		NumberOfClusters: &noc1,

		PrioritizerPolicy: clusterapiv1alpha1.PrioritizerPolicy{
			Mode: clusterapiv1alpha1.PrioritizerPolicyModeExact,
			Configurations: []clusterapiv1alpha1.PrioritizerConfig{
				{
					ScoreCoordinate: &clusterapiv1alpha1.ScoreCoordinate{

						Type: "AddOn",
						AddOn: &clusterapiv1alpha1.AddOnScore{
							ResourceName: "demo",
							ScoreName:    "demo",
						},
					},
					Weight: 1,
				},
				{
					Name:   "ResourceAllocatableCPU",
					Weight: 1,
				},
				{
					Name:   "ResourceAllocatableMemory",
					Weight: 1,
				},
			},
		},
	},
}

func BenchmarkSchedulePlacements100(b *testing.B) {
	benchmarkSchedulePlacements(b, 100)
}

func BenchmarkSchedulePlacements1000(b *testing.B) {
	benchmarkSchedulePlacements(b, 1000)
}

func BenchmarkSchedulePlacements10000(b *testing.B) {
	benchmarkSchedulePlacements(b, 10000)
}

func BenchmarkSchedulePlacements100000(b *testing.B) {
	benchmarkSchedulePlacements(b, 100000)
}

func benchmarkSchedulePlacements(b *testing.B, num int) {
	var err error
	ctx, _ := context.WithCancel(context.Background())
	controllers.ResyncInterval = time.Second * 5

	// start a kube-apiserver
	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join("../../", "deploy", "hub"),
		},
	}

	// prepare client
	if cfg, err = testEnv.Start(); err != nil {
		klog.Fatalf("%v", err)
	}
	if kubeClient, err = kubernetes.NewForConfig(cfg); err != nil {
		klog.Fatalf("%v", err)
	}
	if clusterClient, err = clusterv1client.NewForConfig(cfg); err != nil {
		klog.Fatalf("%v", err)
	}

	// prepare namespace
	createNamespace(namespace)

	go controllers.RunControllerManager(ctx, &controllercmd.ControllerContext{
		KubeConfig:    cfg,
		EventRecorder: util.NewIntegrationTestEventRecorder("integration"),
	})

	go createPlacements(num)
	assertPlacements(num)
}

func createNamespace(namespace string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		klog.Fatalf("%v", err)
	}
}

func createPlacements(num int) {
	for i := 0; i < num; i++ {
		benchmarkPlacement.Name = fmt.Sprintf("%s-%d", name, i)
		_, err := clusterClient.ClusterV1alpha1().Placements(namespace).Create(context.Background(), &benchmarkPlacement, metav1.CreateOptions{})
		if err != nil {
			klog.Fatalf("%v", err)
		}
	}
}

func assertPlacements(num int) {
	for {
		actualNum := 0
		placements, _ := clusterClient.ClusterV1alpha1().Placements(namespace).List(context.Background(), metav1.ListOptions{})
		for _, v := range placements.Items {
			if len(v.Status.Conditions) > 0 {
				actualNum += 1
			}
		}
		if actualNum == num {
			return
		}
		time.Sleep(1 * time.Second)
	}
}
