package http

import "net/http"

// SetupRoutes configures all application endpoints
func SetupRoutes(mux *http.ServeMux) {

	// --- Feature: User Registration ---
	// registerUC := register.NewUsecase(pgPool, redisClient)
	// registerHandler := register.NewHandler(registerUC)
	// mux.HandleFunc("POST /api/v1/register", registerHandler.HTTPv1)
	//
	// --- Feature: User Login ---
	// loginUC := login.NewUsecase(pgPool)
	// loginHandler := login.NewHandler(loginUC)
	// mux.HandleFunc("POST /api/v1/login", loginHandler.HTTPv1)
}
