package scheduling

import (
	"context"
	"fmt"
	"sort"
	"strings"

	kevents "k8s.io/client-go/tools/events"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterlisterv1alpha1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1alpha1"
	clusterapiv1 "open-cluster-management.io/api/cluster/v1"
	clusterapiv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	"open-cluster-management.io/placement/pkg/plugins"
	"open-cluster-management.io/placement/pkg/plugins/addon"
	"open-cluster-management.io/placement/pkg/plugins/balance"
	"open-cluster-management.io/placement/pkg/plugins/predicate"
	"open-cluster-management.io/placement/pkg/plugins/resource"
	"open-cluster-management.io/placement/pkg/plugins/steady"
)

const (
	PrioritizerBalance        string = "Balance"
	PrioritizerSteady         string = "Steady"
	PrioritizerResourcePrefix string = "Resource"

	ScoreCoordinateTypeBuiltIn string = "BuiltIn"
	ScoreCoordinateTypeAddOn   string = "AddOn"
)

// PrioritizerScore defines the score for each cluster
type PrioritizerScore map[string]int64

// Scheduler is an interface for scheduler, it returs the scheduler results
type Scheduler interface {
	Schedule(
		ctx context.Context,
		placement *clusterapiv1alpha1.Placement,
		clusters []*clusterapiv1.ManagedCluster,
	) (ScheduleResult, error)
}

type ScheduleResult interface {
	// FilterResults returns results for each filter
	FilterResults() []FilterResult

	// PrioritizerResults returns results for each prioritizer
	PrioritizerResults() []PrioritizerResult

	// PrioritizerScores returns total score for each cluster
	PrioritizerScores() PrioritizerScore

	// Decisions returns the decisions of the schedule
	Decisions() []clusterapiv1alpha1.ClusterDecision

	// NumOfUnscheduled returns the number of unscheduled.
	NumOfUnscheduled() int
}

type FilterResult struct {
	Name             string   `json:"name"`
	FilteredClusters []string `json:"filteredClusters"`
}

// PrioritizerResult defines the result of one prioritizer,
// include name, weight, and score of each cluster.
type PrioritizerResult struct {
	Name   string           `json:"name"`
	Weight int32            `json:"weight"`
	Scores PrioritizerScore `json:"scores"`
}

// ScheduleResult is the result for a certain schedule.
type scheduleResult struct {
	feasibleClusters     []*clusterapiv1.ManagedCluster
	scheduledDecisions   []clusterapiv1alpha1.ClusterDecision
	unscheduledDecisions int

	filteredRecords map[string][]*clusterapiv1.ManagedCluster
	scoreRecords    []PrioritizerResult
	scoreSum        PrioritizerScore
}

type schedulerHandler struct {
	recorder                kevents.EventRecorder
	placementDecisionLister clusterlisterv1alpha1.PlacementDecisionLister
	scoreLister             clusterlisterv1alpha1.AddOnPlacementScoreLister
	clusterClient           clusterclient.Interface
}

func NewSchedulerHandler(
	clusterClient clusterclient.Interface, placementDecisionLister clusterlisterv1alpha1.PlacementDecisionLister, scoreLister clusterlisterv1alpha1.AddOnPlacementScoreLister, recorder kevents.EventRecorder) plugins.Handle {

	return &schedulerHandler{
		recorder:                recorder,
		placementDecisionLister: placementDecisionLister,
		scoreLister:             scoreLister,
		clusterClient:           clusterClient,
	}
}

func (s *schedulerHandler) EventRecorder() kevents.EventRecorder {
	return s.recorder
}

func (s *schedulerHandler) DecisionLister() clusterlisterv1alpha1.PlacementDecisionLister {
	return s.placementDecisionLister
}

func (s *schedulerHandler) ScoreLister() clusterlisterv1alpha1.AddOnPlacementScoreLister {
	return s.scoreLister
}

func (s *schedulerHandler) ClusterClient() clusterclient.Interface {
	return s.clusterClient
}

// Initialize the default prioritizer weight.
// Balane and Steady weight 1, others weight 0.
// The default weight can be replaced by each placement's PrioritizerConfigs.
var defaultPrioritizerConfig = map[clusterapiv1alpha1.ScoreCoordinate]int32{
	{
		Type:    ScoreCoordinateTypeBuiltIn,
		BuiltIn: PrioritizerBalance,
	}: 1,
	{
		Type:    ScoreCoordinateTypeBuiltIn,
		BuiltIn: PrioritizerSteady,
	}: 1,
}

type pluginScheduler struct {
	handle             plugins.Handle
	filters            []plugins.Filter
	prioritizerWeights map[clusterapiv1alpha1.ScoreCoordinate]int32
}

func NewPluginScheduler(handle plugins.Handle) *pluginScheduler {
	return &pluginScheduler{
		handle: handle,
		filters: []plugins.Filter{
			predicate.New(handle),
		},
		prioritizerWeights: defaultPrioritizerConfig,
	}
}

func (s *pluginScheduler) Schedule(
	ctx context.Context,
	placement *clusterapiv1alpha1.Placement,
	clusters []*clusterapiv1.ManagedCluster,
) (ScheduleResult, error) {
	var err error
	filtered := clusters

	results := &scheduleResult{
		filteredRecords: map[string][]*clusterapiv1.ManagedCluster{},
		scoreRecords:    []PrioritizerResult{},
	}

	// filter clusters
	filterPipline := []string{}

	for _, f := range s.filters {
		filtered, err = f.Filter(ctx, placement, filtered)

		if err != nil {
			return nil, err
		}

		filterPipline = append(filterPipline, f.Name())

		results.filteredRecords[strings.Join(filterPipline, ",")] = filtered
	}

	// Prioritize clusters
	// 1. Get weight for each prioritizers.
	// For example, weights is {"Steady": 1, "Balance":1, "AddOn/default/ratio":3}.
	weights, err := getWeights(s.prioritizerWeights, placement)
	if err != nil {
		return nil, err
	}

	// 2. Generate prioritizers for each placement whose weight != 0.
	prioritizers := getPrioritizers(weights, s.handle)

	// 3. Calculate clusters scores.
	scoreSum := PrioritizerScore{}
	for _, cluster := range filtered {
		scoreSum[cluster.Name] = 0
	}
	for sc, p := range prioritizers {
		// Get cluster score.
		score, err := p.Score(ctx, placement, filtered)
		if err != nil {
			return nil, err
		}

		// Record prioritizer score and weight
		weight := weights[sc]
		results.scoreRecords = append(results.scoreRecords, PrioritizerResult{Name: p.Name(), Weight: weight, Scores: score})

		// The final score is a sum of each prioritizer score * weight.
		// A higher weight indicates that the prioritizer weights more in the cluster selection,
		// while 0 weight indicate thats the prioritizer is disabled.
		for name, val := range score {
			scoreSum[name] = scoreSum[name] + val*int64(weight)
		}

	}

	// 4. Sort clusters by score, if score is equal, sort by name
	sort.SliceStable(filtered, func(i, j int) bool {
		if scoreSum[filtered[i].Name] == scoreSum[filtered[j].Name] {
			return filtered[i].Name < filtered[j].Name
		} else {
			return scoreSum[filtered[i].Name] > scoreSum[filtered[j].Name]
		}
	})

	results.feasibleClusters = filtered
	results.scoreSum = scoreSum

	// select clusters and generate cluster decisions
	// TODO: sort the feasible clusters and make sure the selection stable
	decisions := selectClusters(placement, filtered)
	scheduled, unscheduled := len(decisions), 0
	if placement.Spec.NumberOfClusters != nil {
		unscheduled = int(*placement.Spec.NumberOfClusters) - scheduled
	}
	results.scheduledDecisions = decisions
	results.unscheduledDecisions = unscheduled

	return results, nil
}

// makeClusterDecisions selects clusters based on given cluster slice and then creates
// cluster decisions.
func selectClusters(placement *clusterapiv1alpha1.Placement, clusters []*clusterapiv1.ManagedCluster) []clusterapiv1alpha1.ClusterDecision {
	numOfDecisions := len(clusters)
	if placement.Spec.NumberOfClusters != nil {
		numOfDecisions = int(*placement.Spec.NumberOfClusters)
	}

	// truncate the cluster slice if the desired number of decisions is less than
	// the number of the candidate clusters
	if numOfDecisions < len(clusters) {
		clusters = clusters[:numOfDecisions]
	}

	decisions := []clusterapiv1alpha1.ClusterDecision{}
	for _, cluster := range clusters {
		decisions = append(decisions, clusterapiv1alpha1.ClusterDecision{
			ClusterName: cluster.Name,
		})
	}
	return decisions
}

// Get prioritizer weight for the placement.
// In Additive and "" mode, will override defaultWeight with what placement has defined and return.
// In Exact mode, will return the name and weight defined in placement.
func getWeights(defaultWeight map[clusterapiv1alpha1.ScoreCoordinate]int32, placement *clusterapiv1alpha1.Placement) (map[clusterapiv1alpha1.ScoreCoordinate]int32, error) {
	mode := placement.Spec.PrioritizerPolicy.Mode
	switch {
	case mode == clusterapiv1alpha1.PrioritizerPolicyModeExact:
		return mergeWeights(nil, placement.Spec.PrioritizerPolicy.Configurations), nil
	case mode == clusterapiv1alpha1.PrioritizerPolicyModeAdditive || mode == "":
		return mergeWeights(defaultWeight, placement.Spec.PrioritizerPolicy.Configurations), nil
	default:
		return nil, fmt.Errorf("incorrect prioritizer policy mode: %s", mode)
	}
}

func mergeWeights(defaultWeight map[clusterapiv1alpha1.ScoreCoordinate]int32, customizedWeight []clusterapiv1alpha1.PrioritizerConfig) map[clusterapiv1alpha1.ScoreCoordinate]int32 {
	weights := make(map[clusterapiv1alpha1.ScoreCoordinate]int32)
	// copy the default weight
	for sc, w := range defaultWeight {
		weights[sc] = w
	}

	for _, c := range customizedWeight {
		if c.ScoreCoordinate != nil {
			weights[*c.ScoreCoordinate] = c.Weight
		} else {
			// override default weight
			sc := clusterapiv1alpha1.ScoreCoordinate{
				Type:    ScoreCoordinateTypeBuiltIn,
				BuiltIn: c.Name,
			}
			weights[sc] = c.Weight
		}
	}
	return weights
}

// Generate prioritizers for the placement.
func getPrioritizers(weights map[clusterapiv1alpha1.ScoreCoordinate]int32, handle plugins.Handle) map[clusterapiv1alpha1.ScoreCoordinate]plugins.Prioritizer {
	result := make(map[clusterapiv1alpha1.ScoreCoordinate]plugins.Prioritizer)
	for k, v := range weights {
		if v == 0 {
			continue
		}
		if k.Type == ScoreCoordinateTypeBuiltIn {
			switch {
			case k.BuiltIn == PrioritizerBalance:
				result[k] = balance.New(handle)
			case k.BuiltIn == PrioritizerSteady:
				result[k] = steady.New(handle)
			case strings.HasPrefix(k.BuiltIn, PrioritizerResourcePrefix):
				result[k] = resource.NewResourcePrioritizerBuilder(handle).WithPrioritizerName(k.BuiltIn).Build()
			}
		} else {
			result[k] = addon.NewAddOnPrioritizerBuilder(handle).WithResourceName(k.AddOn.ResourceName).WithScoreName(k.AddOn.ScoreName).Build()
		}
	}
	return result
}

func (r *scheduleResult) FilterResults() []FilterResult {
	results := []FilterResult{}
	for name, r := range r.filteredRecords {
		result := FilterResult{Name: name, FilteredClusters: []string{}}

		for _, c := range r {
			result.FilteredClusters = append(result.FilteredClusters, c.Name)
		}
		results = append(results, result)
	}
	return results
}

func (r *scheduleResult) PrioritizerResults() []PrioritizerResult {
	return r.scoreRecords
}

func (r *scheduleResult) PrioritizerScores() PrioritizerScore {
	return r.scoreSum
}

func (r *scheduleResult) Decisions() []clusterapiv1alpha1.ClusterDecision {
	return r.scheduledDecisions
}

func (r *scheduleResult) NumOfUnscheduled() int {
	return r.unscheduledDecisions
}
