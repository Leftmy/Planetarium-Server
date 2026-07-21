package httpserver

import (
	"log"
	"net/http"
)

// Start runs the HTTP server on the specified port.
func Start(handler http.Handler, port string) {
	log.Printf("Server is starting on port %s...", port)

	// In the future, we will add graceful shutdown logic here
	err := http.ListenAndServe(":"+port, handler)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
