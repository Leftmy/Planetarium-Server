package domain

import "errors"

var (
	// --- (Common) ---
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input data")
	ErrUnauthorized  = errors.New("unauthorized access")

	// --- (User specific) ---
	ErrInvalidEmail = errors.New("invalid email format")
	ErrPasswordWeak = errors.New("password must be at least 8 characters")
	ErrPasswordLong = errors.New("password must be at most 72 bytes")
	ErrInvalidName  = errors.New("display name must be between 1 and 100 characters")

	// ErrInvalidCredentials is deliberately one error for "no such email", "wrong
	// password" and "this account has no password". Separate errors would leak
	// which addresses are registered through the login form.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrInvalidRefreshToken covers unknown, expired and revoked tokens alike.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")

	// ---  (Roadmap specific) ---
	ErrNodeLocked = errors.New("cannot start locked roadmap node")
)
