package main

import (
	"flag"
	"log"
	"maps"
	"sync"
	"time"

	"github.com/mr-diesel/go-metrics/internal/agent"
	models "github.com/mr-diesel/go-metrics/internal/model"
)

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
	addr := flag.String("a", "localhost:8080", "HTTP server address")
	pollInterval := flag.Int("p", 2, "poll interval in seconds")
	reportSeconds := flag.Int("r", 10, "report interval in seconds")
	flag.Parse()
	baseURL := "http://" + *addr
	reportInterval := time.Duration(*reportSeconds) * time.Second
	metrics := &agentMetrics{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	go func() {
		for {
			time.Sleep(time.Duration(*pollInterval) * time.Second)
			metrics.collect()
		}
	}()

	nextReport := time.Now().Add(reportInterval)

	for {
		time.Sleep(time.Until(nextReport))
		nextReport = time.Now().Add(reportInterval)
		err := metrics.report(baseURL)
		if err != nil {
			log.Println(err)
		}
	}
}
