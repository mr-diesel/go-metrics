package handler

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	models "github.com/mr-diesel/go-metrics/internal/model"
	"github.com/mr-diesel/go-metrics/internal/repository"
)

var metricsPage = template.Must(template.New("metrics").Parse(`<!doctype html>
<html lang="ru">
<head><meta charset="utf-8"><title>Metrics</title></head>
<body>
<h1>Metrics</h1>
<ul>
{{range $name, $value := .Gauges}}<li>{{$name}} = {{$value}}</li>{{end}}
{{range $name, $value := .Counters}}<li>{{$name}} = {{$value}}</li>{{end}}
</ul>
</body>
</html>`))

func NewHandler(m repository.Storage) http.Handler {
	router := chi.NewRouter()
	router.Use(routeByEscapedPath)

	update := func(w http.ResponseWriter, r *http.Request) {
		metricType := decodedPathParam(r, "type")
		name := decodedPathParam(r, "name")
		rawValue := decodedPathParam(r, "value")

		if name == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		err := updateMetric(m, metricType, name, rawValue)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
	router.Post("/update/{type}/{name}/{value}", update)
	router.Post("/update/{type}/{name}/", update)
	router.Get("/value/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
		metricType := decodedPathParam(r, "type")
		name := decodedPathParam(r, "name")
		switch metricType {
		case "counter":
			value, ok := m.GetCounter(name)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			io.WriteString(w, strconv.FormatInt(value, 10))
		case "gauge":
			value, ok := m.GetGauge(name)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			io.WriteString(w, strconv.FormatFloat(value, 'g', -1, 64))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Gauges   map[string]float64
			Counters map[string]int64
		}
		data.Gauges, data.Counters = m.Snapshot()

		var page bytes.Buffer
		if err := metricsPage.Execute(&page, data); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(page.Bytes())
	})

	return router
}

func routeByEscapedPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chi.RouteContext(r.Context()).RoutePath = r.URL.EscapedPath()
		next.ServeHTTP(w, r)
	})
}

func decodedPathParam(r *http.Request, key string) string {
	// chi matched a whole segment of URL.EscapedPath, so its escaping is valid.
	value, _ := url.PathUnescape(chi.URLParam(r, key))
	return value
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
