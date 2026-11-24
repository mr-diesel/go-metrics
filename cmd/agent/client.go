package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	models "github.com/mr-diesel/go-metrics/internal/model"
)

var metricsClient = &http.Client{
	Timeout: 3 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func SendMetrics(baseURL string, gauges map[string]float64, counters map[string]int64) (map[string]int64, error) {
	sentCounters := make(map[string]int64)
	var sendErrors []error
	for name, value := range gauges {
		rawValue := strconv.FormatFloat(value, 'g', -1, 64)
		err := SendMetric(baseURL, models.Gauge, name, rawValue)
		if err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("send gauge %s: %w", name, err))
		}
	}
	for name, value := range counters {
		rawValue := strconv.FormatInt(value, 10)
		err := SendMetric(baseURL, models.Counter, name, rawValue)
		if err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("send counter %s: %w", name, err))
			continue
		}
		sentCounters[name] = value
	}
	return sentCounters, errors.Join(sendErrors...)
}

func SendMetric(baseURL, metricType, name, value string) error {
	endpoint := fmt.Sprintf("%s/update/%s/%s/%s", baseURL, metricType, url.PathEscape(name), value)
	resp, err := metricsClient.Post(endpoint, "text/plain", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}
