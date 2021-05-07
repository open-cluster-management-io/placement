package scheduling

import (
	"context"
	"fmt"
	"reflect"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	clusterinformerv1alpha1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1alpha1"
	clusterlisterv1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1"
	clusterlisterv1alpha1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1alpha1"
	clusterapiv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
)

// decisionPlaceholderController creates empty decisions as placeholders in status of PlacementDecisions.
// The contoller support placements with Spec.NumberOfCluster specified only.
// TODO: Add support to placements without Spec.NumberOfCluster specified by enhanceing this controller
// or creating a separate one.
type decisionPlaceholderController struct {
	clusterClient           clusterclient.Interface
	clusterLister           clusterlisterv1.ManagedClusterLister
	placementLister         clusterlisterv1alpha1.PlacementLister
	placementDecisionLister clusterlisterv1alpha1.PlacementDecisionLister
}

// NewDecisionPlaceholderController return an instance of decisionPlaceholderController
func NewDecisionPlaceholderController(
	clusterClient clusterclient.Interface,
	clusterLister clusterlisterv1.ManagedClusterLister,
	placementInformer clusterinformerv1alpha1.PlacementInformer,
	placementDecisionInformer clusterinformerv1alpha1.PlacementDecisionInformer,
	recorder events.Recorder,
) factory.Controller {
	c := decisionPlaceholderController{
		clusterClient:           clusterClient,
		clusterLister:           clusterLister,
		placementLister:         placementInformer.Lister(),
		placementDecisionLister: placementDecisionInformer.Lister(),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			key, _ := cache.MetaNamespaceKeyFunc(obj)
			return key
		}, placementInformer.Informer()).
		WithFilteredEventsInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			labels := accessor.GetLabels()
			placementName := labels[placementLabel]
			return fmt.Sprintf("%s/%s", accessor.GetNamespace(), placementName)
		}, func(obj interface{}) bool {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return false
			}
			labels := accessor.GetLabels()
			if _, ok := labels[placementLabel]; ok {
				return true
			}
			return false
		}, placementDecisionInformer.Informer()).
		WithSync(c.sync).
		ToController("DecisionPlaceholderController", recorder)
}

func (c *decisionPlaceholderController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	queueKey := syncCtx.QueueKey()
	namespace, name, err := cache.SplitMetaNamespaceKey(queueKey)
	if err != nil {
		// ignore placement whose key is not in format: namespace/name
		utilruntime.HandleError(err)
		return nil
	}

	klog.V(4).Infof("Reconciling placement %q", queueKey)
	placement, err := c.placementLister.Placements(namespace).Get(name)
	if errors.IsNotFound(err) {
		// no work if placement is deleted
		return nil
	}
	if err != nil {
		return err
	}

	// no work if placement is deleting
	if !placement.DeletionTimestamp.IsZero() {
		return nil
	}

	placementDecisions, err := getPlacementDecisions(placement.Namespace, placement.Name, c.placementDecisionLister)
	if err != nil {
		return err
	}

	// no work if PlacementDecision has not been created yet
	if len(placementDecisions) == 0 {
		return nil
	}

	// ignore placements without Spec.NumberOfCluster specified
	if placement.Spec.NumberOfClusters == nil {
		return nil
	}

	numOfScheduledDecisions, err := updateClusterDecisions(ctx, placementDecisions, int(*placement.Spec.NumberOfClusters), c.clusterClient)
	if err != nil {
		return err
	}

	// update the status of placement if necessary
	return updatePlacementStatus(ctx, placement, int(*placement.Spec.NumberOfClusters), numOfScheduledDecisions, c.clusterClient)
}

// updateClusterDecisions updates decisions in the status of PlacementDecisions to make sure the
// PlacementDecisions contain desired number of cluster decisions. Empty decision will be appended if necessary.
// It returns the number of decisions which have already been scheduled.
func updateClusterDecisions(ctx context.Context, placementDecisions []*clusterapiv1alpha1.PlacementDecision, desiredNumOfDecisions int, clusterClient clusterclient.Interface) (int, error) {
	// no work if no PlacementDecision has been created yet
	if len(placementDecisions) == 0 {
		return 0, nil
	}

	// TODO: support multiple placementdecisions
	if len(placementDecisions) > 1 {
		return 0, fmt.Errorf("multiple placementdecisions is not supported yet")
	}

	placementDecision := placementDecisions[0]
	newPlacementDecision := placementDecision.DeepCopy()
	decisions := newPlacementDecision.Status.Decisions
	numOfDecisions := len(decisions)

	// truncate decision slice if the number of desisions is larger than the desired NOC
	if numOfDecisions > desiredNumOfDecisions {
		decisions = truncateDecisions(decisions, desiredNumOfDecisions)
	}
	numOfDecisions = len(decisions)

	// append placeholders if necessary
	if numOfDecisions < desiredNumOfDecisions {
		decisions = append(decisions, newPlaceholders(desiredNumOfDecisions-numOfDecisions)...)
	}

	numOfScheduledDecisions := 0
	for _, decison := range decisions {
		if len(decison.ClusterName) != 0 {
			numOfScheduledDecisions++
		}
	}

	// update placementdecision if the decision slice changes
	if reflect.DeepEqual(placementDecision.Status.Decisions, decisions) {
		return numOfScheduledDecisions, nil
	}

	newPlacementDecision.Status.Decisions = decisions
	_, err := clusterClient.ClusterV1alpha1().PlacementDecisions(newPlacementDecision.Namespace).UpdateStatus(ctx, newPlacementDecision, metav1.UpdateOptions{})
	return numOfScheduledDecisions, err
}

// newPlaceholders returns a slice of empty decisons
func newPlaceholders(n int) (decisons []clusterapiv1alpha1.ClusterDecision) {
	for i := 0; i < n; i++ {
		decisons = append(decisons, clusterapiv1alpha1.ClusterDecision{})
	}
	return decisons
}

// truncateDecisions return a truncated decision slice. The number of decisions it contained is no larger than n.
// It will
// 1). Do nothing if the number of decisions is not larger than n;
// 2). And then remove the empty decisons;
// 3). If the number of decisions is still larger than n, the first n decisions will be returned;
func truncateDecisions(decisions []clusterapiv1alpha1.ClusterDecision, n int) []clusterapiv1alpha1.ClusterDecision {
	if len(decisions) <= n {
		return decisions
	}

	newDecisions := []clusterapiv1alpha1.ClusterDecision{}
	for _, decision := range decisions {
		if len(decision.ClusterName) == 0 {
			continue
		}
		newDecisions = append(newDecisions, decision)
	}

	if len(newDecisions) > n {
		newDecisions = newDecisions[:n]
	}

	return newDecisions
}

// updatePlacementStatus updates the status of placement according to numOfDecisions and numOfSelectedClusters.
func updatePlacementStatus(ctx context.Context, placement *clusterapiv1alpha1.Placement, numOfDecisions, numOfSelectedClusters int, clusterClient clusterclient.Interface) error {
	newPlacement := placement.DeepCopy()
	newPlacement.Status.NumberOfSelectedClusters = int32(numOfSelectedClusters)
	satisfiedCondition := newSatisfiedCondition(numOfDecisions, numOfSelectedClusters)
	meta.SetStatusCondition(&newPlacement.Status.Conditions, satisfiedCondition)
	if reflect.DeepEqual(newPlacement.Status, placement.Status) {
		return nil
	}
	_, err := clusterClient.ClusterV1alpha1().Placements(newPlacement.Namespace).UpdateStatus(ctx, newPlacement, metav1.UpdateOptions{})
	return err
}

// newSatisfiedCondition returns a new condition with type PlacementConditionSatisfied
func newSatisfiedCondition(numbOfDecisions, numbOfScheduledDecisions int) metav1.Condition {
	condition := metav1.Condition{
		Type: clusterapiv1alpha1.PlacementConditionSatisfied,
	}
	switch {
	case numbOfDecisions == numbOfScheduledDecisions:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "AllDecisionsScheduled"
		condition.Message = "All cluster decisions scheduled"
	default:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NotAllDecisionsScheduled"
		condition.Message = fmt.Sprintf("%d cluster decisions unscheduled", numbOfDecisions-numbOfScheduledDecisions)
	}
	return condition
}
