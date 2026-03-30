package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Service: "lista-da-vez-api",
			Status:  "ok",
		})
	})

	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("lista-da-vez api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
