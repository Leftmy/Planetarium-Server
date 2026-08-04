// Package httperr turns domain errors into the single error shape the API
// promises.
//
// It is a package of its own rather than a file in transport/http because the
// feature slices need it and transport/http imports the slices; putting it there
// would be an import cycle.
package httperr

import (
	"errors"
	"log"
	"net/http"

	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/pkg/render"
)

// Body is the response every failing endpoint returns. One shape across the API
// is the whole point: a client that has to handle two error formats from two
// neighbouring endpoints will get one of them wrong.
type Body struct {
	Error Detail `json:"error"`
}

// Detail carries a stable machine-readable code and a human-readable message.
//
// Code is what a client branches on. Message is English and aimed at developers
// and logs — user-facing text is the client's job, chosen from the code, because
// only the client knows the user's language.
type Detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// mapping is the contract agreed in docs/api-contract-draft.md. Anything absent
// from it is a bug rather than a case the client can handle, and becomes a 500.
var mapping = map[error]Detail{
	domain.ErrInvalidInput:  {Code: "invalid_request", Message: "Request body is invalid"},
	domain.ErrInvalidEmail:  {Code: "invalid_email", Message: "Email address is not valid"},
	domain.ErrPasswordWeak:  {Code: "password_too_weak", Message: "Password must be at least 8 characters"},
	domain.ErrPasswordLong:  {Code: "password_too_weak", Message: "Password must be at most 72 bytes"},
	domain.ErrInvalidName:   {Code: "invalid_request", Message: "Display name must be between 1 and 100 characters"},
	domain.ErrAlreadyExists: {Code: "email_already_exists", Message: "User with this email already exists"},
	domain.ErrNotFound:      {Code: "not_found", Message: "Resource not found"},
	domain.ErrUnauthorized:  {Code: "unauthorized", Message: "Authentication required"},

	domain.ErrInvalidCredentials:  {Code: "invalid_credentials", Message: "Invalid email or password"},
	domain.ErrInvalidRefreshToken: {Code: "invalid_refresh_token", Message: "Refresh token is invalid, expired or already used"},
}

var status = map[string]int{
	"invalid_request":       http.StatusBadRequest,
	"invalid_email":         http.StatusBadRequest,
	"password_too_weak":     http.StatusBadRequest,
	"not_found":             http.StatusNotFound,
	"unauthorized":          http.StatusUnauthorized,
	"invalid_credentials":   http.StatusUnauthorized,
	"invalid_refresh_token": http.StatusUnauthorized,
	"email_already_exists":  http.StatusConflict,
}

// Write sends err in the agreed shape, choosing the status code from the error
// rather than from the caller — so the same domain error cannot become a 400 in
// one handler and a 409 in another.
//
// An unrecognised error is a 500 whose details go to the log and not to the
// client: an internal message can name tables, queries or file paths.
func Write(w http.ResponseWriter, err error) {
	for domainErr, detail := range mapping {
		if errors.Is(err, domainErr) {
			render.JSON(w, Body{Error: detail}, status[detail.Code])

			return
		}
	}

	log.Printf("unhandled error: %v", err)

	render.JSON(w, Body{Error: Detail{
		Code:    "internal_error",
		Message: "Something went wrong",
	}}, http.StatusInternalServerError)
}
