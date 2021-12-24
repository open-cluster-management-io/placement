package addon

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	"open-cluster-management.io/placement/pkg/plugins"
)

const (
	placementLabel = clusterapiv1alpha1.PlacementLabel
	description    = `
	Customize prioritizer get cluster scores from AddOnPlacementScores with sepcific
	resource name and score name. The clusters which doesn't have corresponding 
	AddOnPlacementScores resource or has expired score is given score 0.
	`
)

var _ plugins.Prioritizer = &AddOn{}

type AddOn struct {
	handle          plugins.Handle
	prioritizerName string
	resourceName    string
	scoreName       string
}

type AddOnBuilder struct {
	addOn *AddOn
}

func NewAddOnPrioritizerBuilder(handle plugins.Handle) *AddOnBuilder {
	return &AddOnBuilder{
		addOn: &AddOn{
			handle: handle,
		},
	}
}

func (c *AddOnBuilder) WithResourceName(name string) *AddOnBuilder {
	c.addOn.resourceName = name
	return c
}

func (c *AddOnBuilder) WithScoreName(name string) *AddOnBuilder {
	c.addOn.scoreName = name
	return c
}

func (c *AddOnBuilder) Build() *AddOn {
	c.addOn.prioritizerName = "AddOn" + "/" + c.addOn.resourceName + "/" + c.addOn.scoreName
	return c.addOn
}

func (c *AddOn) Name() string {
	return c.prioritizerName
}

func (c *AddOn) Description() string {
	return description
}

func (c *AddOn) Score(ctx context.Context, placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) (map[string]int64, error) {
	scores := map[string]int64{}
	expiredScores := ""

	for _, cluster := range clusters {
		namespace := cluster.Name
		// default score is 0
		scores[cluster.Name] = 0

		// get AddOnPlacementScores CR with resourceName
		addOnScores, err := c.handle.ScoreLister().AddOnPlacementScores(namespace).Get(c.resourceName)
		if err != nil {
			klog.Warningf("Getting AddOnPlacementScores failed :%s", err)
			continue
		}

		// check socre valid time
		if (addOnScores.Status.ValidUntil != nil) && time.Now().After(addOnScores.Status.ValidUntil.Time) {
			expiredScores = fmt.Sprintf("%s %s/%s", expiredScores, namespace, c.resourceName)
			continue
		}

		// get AddOnPlacementScores score with scoreName
		for _, v := range addOnScores.Status.Scores {
			if v.Name == c.scoreName {
				scores[cluster.Name] = int64(v.Value)
			}
		}
	}

	if len(expiredScores) > 0 {
		c.handle.EventRecorder().Eventf(
			placement, nil, corev1.EventTypeWarning,
			"AddOnPlacementScoresExpired", "AddOnPlacementScoresExpired",
			"AddOnPlacementScores%s expired", expiredScores)
	}

	return scores, nil
}
