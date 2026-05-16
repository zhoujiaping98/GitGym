package main

import (
	"log"
	"net/http"

	httpx "gitgym/services/runner/internal/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":8081", httpx.NewRouter()))
}
