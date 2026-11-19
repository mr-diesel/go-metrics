package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	models "github.com/mr-diesel/go-metrics/internal/model"
	"github.com/mr-diesel/go-metrics/internal/repository"
)

func NewHandler(m repository.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")

		if len(parts) != 5 || parts[3] == "" || parts[1] != "update" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			w.WriteHeader(http.StatusMethodNotAllowed)
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

func updateMetric(m repository.Storage, metricType, name, rawValue string) error {
	switch metricType {
	case models.Gauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("invalid gauge value")
		}
		m.SetGauge(name, value)
	case models.Counter:
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
