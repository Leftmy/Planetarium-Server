package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("Loading configuration...")
	log.Println("Initializing logger...")
	log.Println("Connecting to adapters...")

	// 1. Create router
	mux := http.NewServeMux()

	// http_transport.SetupRoutes(mux)
	log.Println("Routes configured...")

	// httpserver.Start(mux, "8080")
	log.Println("Application is running. Press Ctrl+C to exit.")

	_ = mux
}
