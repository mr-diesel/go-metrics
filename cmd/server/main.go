package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/mr-diesel/go-metrics/internal/handler"
	"github.com/mr-diesel/go-metrics/internal/repository"
)

func main() {
	addr := flag.String("a", "localhost:8080", "HTTP server address")
	flag.Parse()
	storage := repository.NewMemStorage()
	metricsHandler := handler.NewHandler(storage)

	err := http.ListenAndServe(*addr, metricsHandler)
	if err != nil {
		log.Fatal(err)
	}
}
