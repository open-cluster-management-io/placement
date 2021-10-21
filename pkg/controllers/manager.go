package hub

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterscheme "open-cluster-management.io/api/client/cluster/clientset/versioned/scheme"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	scheduling "open-cluster-management.io/placement/pkg/controllers/scheduling"
	"open-cluster-management.io/placement/pkg/debugger"
)

// RunControllerManager starts the controllers on hub to make placement decisions.
func RunControllerManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	clusterClient, err := clusterclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterInformers := clusterinformers.NewSharedInformerFactory(clusterClient, 10*time.Minute)

	broadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: kubeClient.EventsV1()})

	broadcaster.StartRecordingToSink(ctx.Done())

	recorder := broadcaster.NewRecorder(clusterscheme.Scheme, "placementController")

	scheduler := scheduling.NewPluginScheduler(
		scheduling.NewSchedulerHandler(
			clusterClient,
			kubeClient,
			clusterInformers.Cluster().V1alpha1().PlacementDecisions().Lister(),
			recorder),
	)

	if controllerContext.Server != nil {
		debug := debugger.NewDebugger(
			scheduler,
			clusterInformers.Cluster().V1alpha1().Placements(),
			clusterInformers.Cluster().V1().ManagedClusters(),
		)

		installDebugger(controllerContext.Server.Handler.NonGoRestfulMux, debug)
	}

	schedulingController := scheduling.NewSchedulingController(
		clusterClient,
		clusterInformers.Cluster().V1().ManagedClusters(),
		clusterInformers.Cluster().V1beta1().ManagedClusterSets(),
		clusterInformers.Cluster().V1beta1().ManagedClusterSetBindings(),
		clusterInformers.Cluster().V1alpha1().Placements(),
		clusterInformers.Cluster().V1alpha1().PlacementDecisions(),
		scheduler,
		controllerContext.EventRecorder, recorder,
	)

	go clusterInformers.Start(ctx.Done())

	go schedulingController.Run(ctx, 1)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	go houseclean(clusterInformers, clusterClient, ticker)

	<-ctx.Done()
	return nil
}

func installDebugger(mux *mux.PathRecorderMux, d *debugger.Debugger) {
	mux.HandlePrefix(debugger.DebugPath, http.HandlerFunc(d.Handler))
}

func houseclean(clusterInformers clusterinformers.SharedInformerFactory, clusterClient *clusterclient.Clientset, ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
			klog.Infof("House cleaning ...")
			prioritizerCount := map[string]int64{}

			placments, _ := clusterInformers.Cluster().V1alpha1().Placements().Lister().List(labels.Everything())
			managedClusterScalars, _ := clusterInformers.Cluster().V1alpha1().ManagedClusterScalars().Lister().List(labels.Everything())

			for _, p := range placments {
				for _, c := range p.Spec.PrioritizerPolicy.Configurations {
					if strings.HasPrefix(c.Name, "Customize") {
						prioritizerCount[c.Name] += 1
					}
				}
			}

			for _, m := range managedClusterScalars {
				if _, exist := prioritizerCount[m.Spec.PrioritizerName]; !exist {
					klog.Infof("Cleaning ManagedClusterScalars %s in namespace %s", m.Name, m.Namespace)
					err := clusterClient.ClusterV1alpha1().ManagedClusterScalars(m.Namespace).Delete(context.Background(), m.Name, metav1.DeleteOptions{})
					if err != nil {
						klog.Warningf("Failed to clean ManagedClusterScalars : %s", err)
					}
				}
			}
		}
	}
}
