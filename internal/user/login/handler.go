package login

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/internal/transport/httperr"
	"github.com/leftmy/planetarium-server/pkg/render"
)

type loginUsecase interface {
	Login(ctx context.Context, req Request) (Response, error)
	Refresh(ctx context.Context, req RefreshRequest) (Response, error)
}

type Handler struct {
	usecase loginUsecase
}

func NewHandler(uc loginUsecase) *Handler {
	return &Handler{usecase: uc}
}

func (h *Handler) HTTPv1(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	var req Request

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, domain.ErrInvalidInput)

		return
	}

	resp, err := h.usecase.Login(r.Context(), req)
	if err != nil {
		httperr.Write(w, err)

		return
	}

	render.JSON(w, resp, http.StatusOK)
}

// RefreshHTTPv1 serves POST /api/v1/refresh. It lives in this slice because a
// refresh is the same act as a login — issuing a session — with a different
// credential.
func (h *Handler) RefreshHTTPv1(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	var req RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, domain.ErrInvalidInput)

		return
	}

	resp, err := h.usecase.Refresh(r.Context(), req)
	if err != nil {
		httperr.Write(w, err)

		return
	}

	render.JSON(w, resp, http.StatusOK)
}
