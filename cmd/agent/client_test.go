package main

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

func TestSendMetrics_ReturnsOnlySentCounters(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantErr  bool
		wantSent map[string]int64
	}{
		{"HTTP 500", http.StatusInternalServerError, true, nil},
		{"HTTP 200", http.StatusOK, false, map[string]int64{models.PollCount: 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				const wantPath = "/update/counter/PollCount/5"
				if got := r.URL.Path; got != wantPath {
					t.Errorf("path = %q, want %q", got, wantPath)
				}

				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			counters := map[string]int64{models.PollCount: 5}
			sentCounters, err := SendMetrics(t.Context(), server.URL, nil, counters)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendMetrics returned err = %v, wantErr = %t", err, tt.wantErr)
			}
			if !maps.Equal(sentCounters, tt.wantSent) {
				t.Errorf("sent counters = %v, want %v", sentCounters, tt.wantSent)
			}
			if got := counters[models.PollCount]; got != 5 {
				t.Errorf("input PollCount = %d, want 5", got)
			}
		})
	}
}

func TestSendMetrics_RejectsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/update/counter/PollCount/5":
			http.Redirect(w, r, "/receipt", http.StatusFound)
		case "/receipt":
			t.Errorf("client followed redirect: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	counters := map[string]int64{models.PollCount: 5}
	sentCounters, err := SendMetrics(t.Context(), server.URL, nil, counters)
	if err == nil {
		t.Error("SendMetrics returned nil after redirect")
	}
	if len(sentCounters) != 0 {
		t.Errorf("sent counters after redirect = %v, want none", sentCounters)
	}
}

func TestSendMetrics_PostsGaugeAsPlainText(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		const wantPath = "/update/gauge/room%2Ftemperature%252F/12.5"
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if got := r.URL.EscapedPath(); got != wantPath {
			t.Errorf("path = %q, want %q", got, wantPath)
		}
		const wantContentType = "text/plain"
		if got := r.Header.Get("Content-Type"); got != wantContentType {
			t.Errorf("Content-Type = %q, want %q", got, wantContentType)
		}

		w.WriteHeader(http.StatusOK)
	}))
	gauges := map[string]float64{"room/temperature%2F": 12.5}
	_, err := SendMetrics(t.Context(), server.URL, gauges, nil)
	server.Close()

	if err != nil {
		t.Fatalf("SendMetrics: %v", err)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
}

func TestSendMetrics_SendsGaugeWhenCounterFails(t *testing.T) {
	var gaugeReceived, counterReceived bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/update/gauge/Temperature/12.5":
			gaugeReceived = true
			w.WriteHeader(http.StatusOK)
		case "/update/counter/PollCount/5":
			counterReceived = true
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	gauges := map[string]float64{"Temperature": 12.5}
	counters := map[string]int64{models.PollCount: 5}
	sentCounters, err := SendMetrics(t.Context(), server.URL, gauges, counters)
	server.Close()

	if !gaugeReceived || !counterReceived {
		t.Errorf("received gauge = %t, counter = %t; want both", gaugeReceived, counterReceived)
	}
	if err == nil {
		t.Error("SendMetrics returned nil after counter failed")
	}
	if len(sentCounters) != 0 {
		t.Errorf("sent counters = %v, want none", sentCounters)
	}
}

func TestSendMetrics_ReturnsSentCountersOnDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/update/counter/PollCount/5":
			w.WriteHeader(http.StatusOK)
		case "/update/gauge/Temperature/12.5":
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
				w.WriteHeader(http.StatusOK)
			}
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	gauges := map[string]float64{"Temperature": 12.5}
	counters := map[string]int64{models.PollCount: 5}
	sentCounters, err := SendMetrics(ctx, server.URL, gauges, counters)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("SendMetrics error = %v, want deadline exceeded", err)
	}
	if got := sentCounters[models.PollCount]; got != 5 {
		t.Errorf("sent PollCount = %d, want 5", got)
	}
}

func TestSendMetrics_ReportsAllErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	const gaugeName = "Temperature"
	gauges := map[string]float64{gaugeName: 12.5}
	counters := map[string]int64{models.PollCount: 5}
	_, err := SendMetrics(t.Context(), server.URL, gauges, counters)
	if err == nil {
		t.Fatal("SendMetrics returned nil after both requests failed")
	}
	for _, name := range []string{gaugeName, models.PollCount} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q is missing metric %q", err, name)
		}
	}
}
