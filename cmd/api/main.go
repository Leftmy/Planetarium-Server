package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/leftmy/planetarium-server/config"
	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	httptransport "github.com/leftmy/planetarium-server/internal/transport/http"
	"github.com/leftmy/planetarium-server/pkg/httpserver"
)

func main() {
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	// Connecting here rather than lazily means a wrong DSN stops the process
	// at startup instead of surfacing as a failed request later.
	db, err := postgres.New(context.Background(), cfg.Postgres)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprint(w, "Welcome to the Planetarium API!"); err != nil {
			log.Printf("write response: %v", err)
		}
	})

	httptransport.SetupRoutes(mux)

	httpserver.Start(mux, cfg.AppPort)
}
