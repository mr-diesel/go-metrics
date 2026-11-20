package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

func TestSendMetrics_ResetsPollCountOnlyOnSuccess(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantErr   bool
		wantCount int64
	}{
		{"HTTP 500", http.StatusInternalServerError, true, 5},
		{"HTTP 200", http.StatusOK, false, 0},
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
			err := SendMetrics(server.URL, nil, counters)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendMetrics returned err = %v, wantErr = %t", err, tt.wantErr)
			}
			if got := counters[models.PollCount]; got != tt.wantCount {
				t.Errorf("PollCount = %d, want %d", got, tt.wantCount)
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
	if err := SendMetrics(server.URL, nil, counters); err == nil {
		t.Error("SendMetrics returned nil after redirect")
	}
	if got := counters[models.PollCount]; got != 5 {
		t.Errorf("PollCount = %d, want 5", got)
	}
}

func TestSendMetrics_PostsGaugeAsPlainText(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		const wantPath = "/update/gauge/Temperature/12.5"
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if got := r.URL.Path; got != wantPath {
			t.Errorf("path = %q, want %q", got, wantPath)
		}
		const wantContentType = "text/plain"
		if got := r.Header.Get("Content-Type"); got != wantContentType {
			t.Errorf("Content-Type = %q, want %q", got, wantContentType)
		}

		w.WriteHeader(http.StatusOK)
	}))
	gauges := map[string]float64{"Temperature": 12.5}
	err := SendMetrics(server.URL, gauges, nil)
	server.Close()

	if err != nil {
		t.Fatalf("SendMetrics: %v", err)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
}

func TestSendMetrics_ContinuesAfterGaugeError(t *testing.T) {
	var gaugeReceived, counterReceived bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/update/gauge/Temperature/12.5":
			gaugeReceived = true
			w.WriteHeader(http.StatusInternalServerError)
		case "/update/counter/PollCount/5":
			counterReceived = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	gauges := map[string]float64{"Temperature": 12.5}
	counters := map[string]int64{models.PollCount: 5}
	err := SendMetrics(server.URL, gauges, counters)
	server.Close()

	if !gaugeReceived || !counterReceived {
		t.Errorf("received gauge = %t, counter = %t; want both", gaugeReceived, counterReceived)
	}
	if err == nil {
		t.Error("SendMetrics returned nil after gauge failed")
	}
	if got := counters[models.PollCount]; got != 0 {
		t.Errorf("PollCount = %d, want 0", got)
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
	err := SendMetrics(server.URL, gauges, counters)
	if err == nil {
		t.Fatal("SendMetrics returned nil after both requests failed")
	}
	for _, name := range []string{gaugeName, models.PollCount} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q is missing metric %q", err, name)
		}
	}
}
