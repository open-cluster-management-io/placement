package addon

import (
	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	"open-cluster-management.io/placement/pkg/plugins"
)

const (
	placementLabel = clusterapiv1alpha1.PlacementLabel
	description    = `
	Customize prioritizer xxxxx.
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

func (c *AddOnBuilder) WithPrioritizerName(name string) *AddOnBuilder {
	names := strings.Split(name, "/")
	c.addOn.prioritizerName = name
	if len(names) == 3 {
		c.addOn.resourceName = names[1]
		c.addOn.scoreName = names[2]
	}
	return c
}

func (c *AddOnBuilder) Build() *AddOn {
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

	for _, cluster := range clusters {
		namespace := cluster.Name
		// default score is 0
		scores[cluster.Name] = 0

		// get AddOnPlacementScores CR with resourceName
		addOnScores, err := c.handle.ClusterClient().ClusterV1alpha1().AddOnPlacementScores(namespace).Get(context.Background(), c.resourceName, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Getting AddOnPlacementScores failed :%s", err)
			continue
		}

		// check socre valid time
		if (addOnScores.Status.ValidUntil != nil) && time.Now().After(addOnScores.Status.ValidUntil.Time) {
			klog.Warningf("AddOnPlacementScores %s ValidUntil %s exprired.", c.resourceName, addOnScores.Status.ValidUntil)
			continue
		}

		// get AddOnPlacementScores score with scoreName
		for _, v := range addOnScores.Status.Scores {
			if v.Name == c.scoreName {
				scores[cluster.Name] = int64(v.Value)
			}
		}
	}

	return scores, nil
}
