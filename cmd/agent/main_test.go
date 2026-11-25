package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

func TestRunReports_SendsGaugesWhileCounterIsSlow(t *testing.T) {
	const (
		interval    = 250 * time.Millisecond
		counterPath = "/update/counter/PollCount/5"
		gaugePath   = "/update/gauge/Temperature/12.5"
	)
	metrics := &agentMetrics{
		gauges:   map[string]float64{"Temperature": 12.5},
		counters: map[string]int64{models.PollCount: 5},
	}
	var mu sync.Mutex
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests[r.URL.Path]++
		mu.Unlock()

		if r.URL.Path == counterPath {
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Stop between the second and third reports, before the per-request timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 2*interval+interval/2)
	defer cancel()
	metrics.runReports(ctx, server.URL, interval)
	server.Close()

	for _, path := range []string{counterPath, gaugePath} {
		if got := requests[path]; got != 2 {
			t.Errorf("requests to %s = %d, want 2", path, got)
		}
	}
}

func TestReport_PreservesNewPolls(t *testing.T) {
	metrics := &agentMetrics{
		gauges:   make(map[string]float64),
		counters: map[string]int64{models.PollCount: 5},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		metrics.collect()
		metrics.collect()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := metrics.report(t.Context(), server.URL); err != nil {
		t.Fatalf("report: %v", err)
	}
	_, counters := metrics.snapshot()
	if got := counters[models.PollCount]; got != 2 {
		t.Errorf("PollCount = %d, want 2", got)
	}
}

func TestReport_DeductsPollCountAfterGaugeError(t *testing.T) {
	metrics := &agentMetrics{
		gauges:   map[string]float64{"Temperature": 12.5},
		counters: map[string]int64{models.PollCount: 5},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/update/gauge/Temperature/12.5":
			w.WriteHeader(http.StatusInternalServerError)
		case "/update/counter/PollCount/5":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := metrics.report(t.Context(), server.URL); err == nil {
		t.Error("report returned nil after gauge failed")
	}
	_, counters := metrics.snapshot()
	if got := counters[models.PollCount]; got != 0 {
		t.Errorf("PollCount = %d, want 0", got)
	}
}

func TestReport_KeepsPollCountAfterCounterError(t *testing.T) {
	metrics := &agentMetrics{counters: map[string]int64{models.PollCount: 5}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := metrics.report(t.Context(), server.URL); err == nil {
		t.Fatal("report returned nil after counter failed")
	}
	_, counters := metrics.snapshot()
	if got := counters[models.PollCount]; got != 5 {
		t.Errorf("PollCount = %d, want 5", got)
	}
}
