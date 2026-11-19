package agent

import (
	"testing"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

func TestCollectMetrics(t *testing.T) {
	const (
		missingMetricFormat = "metric missing %q"
		mismatchFormat      = "%s = %d, expected %d"

		wantGauges = 28
	)

	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	CollectMetrics(gauges, counters)
	if got := counters[models.PollCount]; got != 1 {
		t.Fatalf(mismatchFormat, models.PollCount, got, 1)
	}
	for _, name := range models.RuntimeGaugeNames {
		if _, ok := gauges[name]; !ok {
			t.Errorf(missingMetricFormat, name)
		}
	}
	if _, ok := gauges[models.RandomValue]; !ok {
		t.Errorf(missingMetricFormat, models.RandomValue)
	}
	if got := len(gauges); got != wantGauges {
		t.Errorf(mismatchFormat, "len(gauges)", got, wantGauges)
	}

	CollectMetrics(gauges, counters)
	if got := counters[models.PollCount]; got != 2 {
		t.Fatalf(mismatchFormat, models.PollCount, got, 2)
	}
	if got := len(gauges); got != wantGauges {
		t.Errorf(mismatchFormat, "len(gauges)", got, wantGauges)
	}
}
