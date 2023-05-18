package scheduling

import (
	"k8s.io/component-base/metrics/legacyregistry"
	testingclock "k8s.io/utils/clock/testing"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	t0 := time.Unix(0, 0)
	c := testingclock.NewFakeClock(t0)

	metrics := newScheduleMetrics(c)

	metrics.startSchedule("test")
	c.Step(50 * time.Second)
	metrics.startBind("test")
	c.Step(30 * time.Second)
	metrics.done("test")

	metrics.startSchedule("test1")
	c.Step(50 * time.Second)
	metrics.startBind("test1")
	c.Step(30 * time.Second)
	metrics.done("test1")

	mfs, err := legacyregistry.DefaultGatherer.Gather()
	if err != nil {
		t.Errorf("failed to gather metrics")
	}

	for _, mf := range mfs {
		if *mf.Name == SchedulingSubsystem+"_"+BindDurationKey {
			mfMetric := mf.GetMetric()
			for _, m := range mfMetric {
				if m.GetHistogram().GetSampleCount() != 2 {
					t.Errorf("sample count is not correct")
				}
			}
		}
	}
}
