package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5_000_000_000,
	}

	log.Printf("MCPProxy Sidekick listening on %s", cfg.ListenAddr)
	log.Fatal(srv.ListenAndServe())
}
