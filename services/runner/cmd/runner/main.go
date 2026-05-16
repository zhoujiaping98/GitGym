package main

import (
	"log"
	"net/http"

	"gitgym/services/runner/internal/config"
	httpx "gitgym/services/runner/internal/http"
)

func main() {
	cfg := config.Load()
	log.Fatal(http.ListenAndServe(":8081", httpx.NewRouter(cfg.WorkRoot)))
}
