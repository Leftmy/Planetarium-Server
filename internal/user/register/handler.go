package register

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/internal/transport/httperr"
	"github.com/leftmy/planetarium-server/pkg/render"
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
	defer func() { _ = r.Body.Close() }()

	var req Request

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, domain.ErrInvalidInput)

		return
	}

	resp, err := h.usecase.Register(r.Context(), req)
	if err != nil {
		// Every failure goes through the same mapping, so a duplicate email is a
		// 409 here and everywhere else rather than whatever this handler decides.
		httperr.Write(w, err)

		return
	}

	render.JSON(w, resp, http.StatusCreated)
}
