package main

import (
	"fmt"
	"log"
	"net/http"

	httptransport "github.com/leftmy/planetarium-server/internal/transport/http"
	"github.com/leftmy/planetarium-server/pkg/httpserver"
)

// defaultPort stays a constant until config.InitConfig actually parses the environment.
const defaultPort = "8080"

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprint(w, "Welcome to the Planetarium API!"); err != nil {
			log.Printf("write response: %v", err)
		}
	})

	httptransport.SetupRoutes(mux)

	httpserver.Start(mux, defaultPort)
}
