package http

import (
	"fmt"
	"log"
	"net/http"

	"github.com/leftmy/planetarium-server/docs"
	"github.com/leftmy/planetarium-server/internal/user/login"
	"github.com/leftmy/planetarium-server/internal/user/register"
)

// Handlers is what the entrypoint builds and hands over. Passing the handlers in
// rather than constructing them here keeps the wiring in one place — main.go —
// and leaves this file about routing only.
type Handlers struct {
	Register *register.Handler
	Login    *login.Handler
}

// SetupRoutes configures all application endpoints.
func SetupRoutes(mux *http.ServeMux, h Handlers) {
	// "/{$}" matches the root and nothing else. A bare "/" would be a catch-all
	// answering every unknown path with 200 and this greeting, which makes a
	// missing endpoint look like a working one.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprint(w, "Welcome to the Planetarium API!"); err != nil {
			log.Printf("write response: %v", err)
		}
	})

	// --- Feature: User Registration ---
	mux.HandleFunc("POST /api/v1/register", h.Register.HTTPv1)

	// --- Feature: Sessions ---
	mux.HandleFunc("POST /api/v1/login", h.Login.HTTPv1)
	mux.HandleFunc("POST /api/v1/refresh", h.Login.RefreshHTTPv1)

	// --- API documentation ---
	mux.HandleFunc("GET /api/v1/openapi.yaml", serveBytes("application/yaml", docs.OpenAPI))
	mux.HandleFunc("GET /docs", serveBytes("text/html; charset=utf-8", docs.SwaggerUI))
}

// serveBytes returns a handler for one embedded file. The content is fixed at
// build time, so there is nothing to read, parse or fail at request time.
func serveBytes(contentType string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)

		if _, err := w.Write(body); err != nil {
			log.Printf("write documentation response: %v", err)
		}
	}
}
