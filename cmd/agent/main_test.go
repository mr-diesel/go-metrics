package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

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

	if err := metrics.report(server.URL); err != nil {
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

	if err := metrics.report(server.URL); err == nil {
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

	if err := metrics.report(server.URL); err == nil {
		t.Fatal("report returned nil after counter failed")
	}
	_, counters := metrics.snapshot()
	if got := counters[models.PollCount]; got != 5 {
		t.Errorf("PollCount = %d, want 5", got)
	}
}
