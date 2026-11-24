package main

import (
	"flag"
	"log"
	"maps"
	"math"
	"sync"
	"time"

	"github.com/mr-diesel/go-metrics/internal/agent"
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
	sentCounters, err := SendMetrics(baseURL, gauges, counters)

	m.mu.Lock()
	defer m.mu.Unlock()
	for name, value := range sentCounters {
		m.counters[name] -= value
	}
	return err
}

func main() {
	addr := flag.String("a", "localhost:8080", "HTTP server address")
	pollSeconds := flag.Int64("p", 2, "poll interval in seconds")
	reportSeconds := flag.Int64("r", 10, "report interval in seconds")
	flag.Parse()

	const maxIntervalSeconds = math.MaxInt64 / int64(time.Second)
	if *pollSeconds <= 0 || *pollSeconds > maxIntervalSeconds {
		log.Fatalf("invalid -p=%d: poll interval must be between 1 and %d seconds", *pollSeconds, maxIntervalSeconds)
	}
	if *reportSeconds <= 0 || *reportSeconds > maxIntervalSeconds {
		log.Fatalf("invalid -r=%d: report interval must be between 1 and %d seconds", *reportSeconds, maxIntervalSeconds)
	}

	baseURL := "http://" + *addr
	pollInterval := time.Duration(*pollSeconds) * time.Second
	reportInterval := time.Duration(*reportSeconds) * time.Second
	metrics := &agentMetrics{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	go func() {
		for {
			time.Sleep(pollInterval)
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
