package repository

import (
	"testing"
)

func TestMemStorage_SetGauge(t *testing.T) {
	s := NewMemStorage()
	s.SetGauge("Temperature", 12.5)
	s.SetGauge("Temperature", 3.5)
	if got := s.gauges["Temperature"]; got != 3.5 {
		t.Errorf("Temperature = %g, want %g", got, 3.5)
	}
}

func TestMemStorage_AddCounter(t *testing.T) {
	s := NewMemStorage()
	s.AddCounter("Requests", 5)
	s.AddCounter("Requests", 3)
	if got := s.counters["Requests"]; got != 8 {
		t.Errorf("Requests = %d, want %d", got, 8)
	}
}
