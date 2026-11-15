package main

import (
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Storage interface {
	SetGauge(name string, value float64)
	AddCounter(name string, value int64)
}

var _ Storage = (*MemStorage)(nil)

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.Mutex
}

func (m *MemStorage) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

func (m *MemStorage) AddCounter(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func newHandler(m Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		parts := strings.Split(r.URL.Path, "/")

		if len(parts) != 5 || parts[3] == "" || parts[1] != "update" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		err := updateMetric(m, parts[2], parts[3], parts[4])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func updateMetric(m Storage, metricType, name, rawValue string) error {
	switch metricType {
	case "gauge":
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("invalid gauge value")
		}
		m.SetGauge(name, value)
	case "counter":
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return err
		}
		m.AddCounter(name, value)
	default:
		return errors.New("unknown metric type")
	}
	return nil
}

func main() {
	m := NewMemStorage()
	handler := newHandler(m)

	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		log.Fatal(err)
	}
}
