package main

import (
	"log"
	"net/http"

	httpx "gitgym/services/api/internal/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", httpx.NewRouter()))
}
