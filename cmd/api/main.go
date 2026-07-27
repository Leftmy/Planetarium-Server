package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Welcome to the Planetarium API!")
	})

	// 1. Create router
	mux := http.NewServeMux()

	// http_transport.SetupRoutes(mux)
	log.Println("Routes configured...")

	// httpserver.Start(mux, "8080")
	log.Println("Application is running. Press Ctrl+C to exit.")

	_ = mux

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
