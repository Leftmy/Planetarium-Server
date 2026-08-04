package render

import (
	"encoding/json"
	"log"
	"net/http"
)

func JSON(w http.ResponseWriter, body any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	// The status line is already sent, so there is no way to turn a failure here
	// into an error response — and a broken pipe from a client that hung up is
	// the usual cause. Log it and let the request end.
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("render: encode response: %v", err)
	}
}
