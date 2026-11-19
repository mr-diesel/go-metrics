package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

func (s *storageSpy) AddCounter(name string, value int64) {
	s.counterName = name
	s.counterValue = value
}

func TestNewHandler(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantStorage storageSpy
	}{
		{"method not allowed", http.MethodGet, "/update/gauge/Temperature/12.5", http.StatusMethodNotAllowed, storageSpy{}},
		{"unknown route", http.MethodGet, "/create/gauge/Temperature/12.5", http.StatusNotFound, storageSpy{}},
		{"invalid counter value", http.MethodPost, "/update/counter/PollCount/abc", http.StatusBadRequest, storageSpy{}},
		{"missing metric name", http.MethodPost, "/update/counter//5", http.StatusNotFound, storageSpy{}},
		{"valid gauge", http.MethodPost, "/update/gauge/Temperature/12.5", http.StatusOK, storageSpy{gaugeName: "Temperature", gaugeValue: 12.5}},
		{"valid counter", http.MethodPost, "/update/counter/Requests/5", http.StatusOK, storageSpy{counterName: "Requests", counterValue: 5}},
		{"non-numeric gauge", http.MethodPost, "/update/gauge/Temperature/abc", http.StatusBadRequest, storageSpy{}},
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
