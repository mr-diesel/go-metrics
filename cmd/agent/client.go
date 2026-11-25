package main

import (
	"context"
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

func SendMetrics(ctx context.Context, baseURL string, gauges map[string]float64, counters map[string]int64) (map[string]int64, error) {
	gaugesDone := make(chan error)
	go func() {
		gaugesDone <- sendGauges(ctx, baseURL, gauges)
	}()

	sentCounters, counterErr := sendCounters(ctx, baseURL, counters)
	gaugeErr := <-gaugesDone
	return sentCounters, errors.Join(counterErr, gaugeErr)
}

func sendCounters(ctx context.Context, baseURL string, counters map[string]int64) (map[string]int64, error) {
	sentCounters := make(map[string]int64)
	var sendErrors []error
	for name, value := range counters {
		rawValue := strconv.FormatInt(value, 10)
		err := SendMetric(ctx, baseURL, models.Counter, name, rawValue)
		if err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("send counter %s: %w", name, err))
			if ctx.Err() != nil {
				break
			}
			continue
		}
		sentCounters[name] = value
	}
	return sentCounters, errors.Join(sendErrors...)
}

func sendGauges(ctx context.Context, baseURL string, gauges map[string]float64) error {
	var sendErrors []error
	for name, value := range gauges {
		rawValue := strconv.FormatFloat(value, 'g', -1, 64)
		err := SendMetric(ctx, baseURL, models.Gauge, name, rawValue)
		if err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("send gauge %s: %w", name, err))
			if ctx.Err() != nil {
				break
			}
		}
	}
	return errors.Join(sendErrors...)
}

func SendMetric(ctx context.Context, baseURL, metricType, name, value string) error {
	endpoint := fmt.Sprintf("%s/update/%s/%s/%s", baseURL, metricType, url.PathEscape(name), value)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := metricsClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}
