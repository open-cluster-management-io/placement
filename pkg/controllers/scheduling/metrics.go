package scheduling

import (
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/utils/clock"
	"time"
)

const (
	SchedulingName        = "placement_scheduling"
	SchedulingSubsystem   = "scheduling"
	SchedulingDurationKey = "scheduling_duration_seconds"
	BindDurationKey       = "bind_duration_seconds"
)

var (
	schedulingDuration = k8smetrics.NewHistogramVec(&k8smetrics.HistogramOpts{
		Subsystem:      SchedulingSubsystem,
		Name:           SchedulingDurationKey,
		StabilityLevel: k8smetrics.ALPHA,
		Help:           "How long in seconds schedule a placement.",
		Buckets:        k8smetrics.ExponentialBuckets(10e-7, 10, 10),
	}, []string{"name"})
	bindDuration = k8smetrics.NewHistogramVec(&k8smetrics.HistogramOpts{
		Subsystem:      SchedulingSubsystem,
		Name:           BindDurationKey,
		StabilityLevel: k8smetrics.ALPHA,
		Help:           "How long in seconds bind a placement to placementDecisions.",
		Buckets:        k8smetrics.LinearBuckets(10e-7, 10, 10),
	}, []string{"name"})

	metrics = []k8smetrics.Registerable{
		schedulingDuration, bindDuration,
	}
)

func init() {
	for _, m := range metrics {
		legacyregistry.MustRegister(m)
	}
}

// HistogramMetric counts individual observations.
type HistogramMetric interface {
	Observe(float64)
}

type scheduleMetrics struct {
	clock clock.Clock

	scheduling HistogramMetric
	binding    HistogramMetric

	scheduleStartTimes map[string]time.Time
	bindStartTimes     map[string]time.Time
}

func newScheduleMetrics(clock clock.Clock) *scheduleMetrics {
	return &scheduleMetrics{
		clock:              clock,
		scheduling:         schedulingDuration.WithLabelValues(SchedulingName),
		binding:            bindDuration.WithLabelValues(SchedulingName),
		scheduleStartTimes: map[string]time.Time{},
		bindStartTimes:     map[string]time.Time{},
	}
}

func (m *scheduleMetrics) startSchedule(key string) {
	if m == nil {
		return
	}

	m.scheduleStartTimes[key] = m.clock.Now()
	if _, exists := m.scheduleStartTimes[key]; !exists {
		m.scheduleStartTimes[key] = m.clock.Now()
	}
}

// Gets the time since the specified start in seconds.
func (m *scheduleMetrics) sinceInSeconds(start time.Time) float64 {
	return m.clock.Since(start).Seconds()
}

func (m *scheduleMetrics) startBind(key string) {
	if m == nil {
		return
	}

	m.bindStartTimes[key] = m.clock.Now()
	if startTime, exists := m.scheduleStartTimes[key]; exists {
		m.scheduling.Observe(m.sinceInSeconds(startTime))
		delete(m.scheduleStartTimes, key)
	}
}

func (m *scheduleMetrics) done(key string) {
	if m == nil {
		return
	}

	if startTime, exists := m.bindStartTimes[key]; exists {
		m.binding.Observe(m.sinceInSeconds(startTime))
		delete(m.bindStartTimes, key)
	}
}
