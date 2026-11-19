package main

import (
	"log"
	"net/http"

	"github.com/mr-diesel/go-metrics/internal/handler"
	"github.com/mr-diesel/go-metrics/internal/repository"
)

func main() {
	m := repository.NewMemStorage()
	h := handler.NewHandler(m)

	err := http.ListenAndServe(":8080", h)
	if err != nil {
		log.Fatal(err)
	}
}
