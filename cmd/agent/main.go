package main

import (
	"log"
	"maps"
	"sync"
	"time"

	"github.com/mr-diesel/go-metrics/internal/agent"
	models "github.com/mr-diesel/go-metrics/internal/model"
)

const defaultBaseURL = "http://localhost:8080"

type agentMetrics struct {
	mu       sync.Mutex
	gauges   map[string]float64
	counters map[string]int64
}

func (m *agentMetrics) collect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent.CollectMetrics(m.gauges, m.counters)
}

func (m *agentMetrics) snapshot() (map[string]float64, map[string]int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.gauges), maps.Clone(m.counters)
}

func (m *agentMetrics) report(baseURL string) error {
	gauges, counters := m.snapshot()
	pollCount := counters[models.PollCount]
	err := SendMetrics(baseURL, gauges, counters)
	if pollCount > 0 && counters[models.PollCount] == 0 {
		m.mu.Lock()
		m.counters[models.PollCount] -= pollCount
		m.mu.Unlock()
	}
	return err
}

func main() {
	metrics := &agentMetrics{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	go func() {
		for {
			time.Sleep(2 * time.Second)
			metrics.collect()
		}
	}()

	nextReport := time.Now().Add(10 * time.Second)

	for {
		time.Sleep(time.Until(nextReport))
		nextReport = time.Now().Add(10 * time.Second)
		err := metrics.report(defaultBaseURL)
		if err != nil {
			log.Println(err)
		}
	}
}
