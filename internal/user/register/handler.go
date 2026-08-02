package register

import (
	"context"
	"encoding/json"
	"net/http"
	//"github.com/leftmy/planetarium-server"
)

type registerUsecase interface {
	Register(ctx context.Context, req Request) (Response, error)
}

type Handler struct {
	usecase registerUsecase
}

func NewHandler(uc registerUsecase) *Handler {
	return &Handler{
		usecase: uc,
	}
}

func (h *Handler) HTTPv1(w http.ResponseWriter, r *http.Request) {
	var req Request

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := h.usecase.Register(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// render.JSON(w, resp, http.StatusCreated)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
