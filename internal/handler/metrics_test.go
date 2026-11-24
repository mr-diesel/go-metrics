package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mr-diesel/go-metrics/internal/repository"
)

type storageSpy struct {
	gaugeName    string
	gaugeValue   float64
	counterName  string
	counterValue int64
}

func (s *storageSpy) SetGauge(name string, value float64) {
	s.gaugeName = name
	s.gaugeValue = value
}
func (s *storageSpy) GetGauge(name string) (float64, bool) {
	if s.gaugeName == name {
		return s.gaugeValue, true
	}

	return 0, false
}

func (s *storageSpy) AddCounter(name string, value int64) {
	s.counterName = name
	s.counterValue = value
}

func (s *storageSpy) GetCounter(name string) (int64, bool) {
	if s.counterName == name {
		return s.counterValue, true
	}

	return 0, false
}

func (s *storageSpy) Snapshot() (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)
	if s.gaugeName != "" {
		gauges[s.gaugeName] = s.gaugeValue
	}
	if s.counterName != "" {
		counters[s.counterName] = s.counterValue
	}
	return gauges, counters
}

func TestNewHandler_UpdateMetric(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantStorage storageSpy
	}{
		{"get on update route", http.MethodGet, "/update/gauge/Temperature/12.5", http.StatusMethodNotAllowed, storageSpy{}},
		{"unknown route", http.MethodGet, "/create/gauge/Temperature/12.5", http.StatusNotFound, storageSpy{}},
		{"non-numeric counter value", http.MethodPost, "/update/counter/PollCount/abc", http.StatusBadRequest, storageSpy{}},
		{"empty counter value", http.MethodPost, "/update/counter/Requests/", http.StatusBadRequest, storageSpy{}},
		{"missing metric name", http.MethodPost, "/update/counter//5", http.StatusNotFound, storageSpy{}},
		{"missing value after encoded name", http.MethodPost, "/update/counter/jobs%2F5", http.StatusNotFound, storageSpy{}},
		{"valid gauge", http.MethodPost, "/update/gauge/Temperature/12.5", http.StatusOK, storageSpy{gaugeName: "Temperature", gaugeValue: 12.5}},
		{"encoded gauge name", http.MethodPost, "/update/gauge/room%2Ftemp/1.5", http.StatusOK, storageSpy{gaugeName: "room/temp", gaugeValue: 1.5}},
		{"valid counter", http.MethodPost, "/update/counter/Requests/5", http.StatusOK, storageSpy{counterName: "Requests", counterValue: 5}},
		{"non-numeric gauge value", http.MethodPost, "/update/gauge/Temperature/abc", http.StatusBadRequest, storageSpy{}},
		{"unknown metric type", http.MethodPost, "/update/histogram/Temperature/12.5", http.StatusBadRequest, storageSpy{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &storageSpy{}
			h := NewHandler(spy)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Content-Type", "text/plain")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Code; got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusMethodNotAllowed {
				resp := rec.Result()
				defer resp.Body.Close()

				if got := resp.Header.Get("Allow"); got != http.MethodPost {
					t.Errorf("Allow = %q, want %q", got, http.MethodPost)
				}
			}
			if got := *spy; got != tt.wantStorage {
				t.Errorf("storage = %+v, want %+v", got, tt.wantStorage)
			}
		})
	}
}

func TestNewHandler_GetValue(t *testing.T) {
	h := NewHandler(&storageSpy{
		gaugeName:   "Temperature",
		gaugeValue:  12.5,
		counterName: "Requests",
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"stored gauge", "/value/gauge/Temperature", http.StatusOK, "12.5"},
		{"zero counter", "/value/counter/Requests", http.StatusOK, "0"},
		{"encoded counter name", "/value/counter/%52equests", http.StatusOK, "0"},
		{"missing counter", "/value/counter/Missing", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Code; got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestNewHandler_EncodedMetricNames(t *testing.T) {
	server := httptest.NewServer(NewHandler(repository.NewMemStorage()))
	defer server.Close()
	client := server.Client()

	metrics := []struct {
		writeName, readName, value string
	}{
		{"%52equests", "Requests", "5"},
		{"jobs%2Fdone", "jobs%2fdone", "7"},
		{"jobs%252Fdone", "jobs%252Fdone", "11"},
	}
	for _, metric := range metrics {
		path := "/update/counter/" + metric.writeName + "/" + metric.value
		resp, err := client.Post(server.URL+path, "text/plain", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
	}
	for _, metric := range metrics {
		path := "/value/counter/" + metric.readName
		resp, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || string(body) != metric.value {
			t.Errorf("GET %s: status = %d, body = %q, want %d and %q",
				path, resp.StatusCode, body, http.StatusOK, metric.value)
		}
	}
}

func TestNewHandler_ListMetrics(t *testing.T) {
	tests := []struct {
		name  string
		spy   storageSpy
		items []string
	}{
		{name: "empty storage"},
		{
			name:  "gauge and counter",
			spy:   storageSpy{gaugeName: "A&B", gaugeValue: 12.5, counterName: "Requests", counterValue: 7},
			items: []string{"A&amp;B = 12.5", "Requests = 7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			NewHandler(&tt.spy).ServeHTTP(rec, req)

			if got := rec.Code; got != http.StatusOK {
				t.Fatalf("status = %d, want %d", got, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "<ul>") || strings.Count(body, "<li>") != len(tt.items) {
				t.Errorf("want a list with %d items, got %q", len(tt.items), body)
			}
			for _, item := range tt.items {
				if !strings.Contains(body, item) {
					t.Errorf("missing metric %q in %q", item, body)
				}
			}
		})
	}
}
