package agent

import (
	"math/rand"
	"reflect"
	"runtime"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

func CollectMetrics(gauges map[string]float64, counters map[string]int64) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	statsValue := reflect.ValueOf(&stats).Elem()
	for _, name := range models.RuntimeGaugeNames {
		field := statsValue.FieldByName(name)
		gauges[name] = field.Convert(reflect.TypeOf(float64(0))).Float()
	}

	gauges[models.RandomValue] = rand.Float64()
	counters[models.PollCount]++
}
