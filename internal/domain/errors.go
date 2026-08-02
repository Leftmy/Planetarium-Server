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

	// ---  (Roadmap specific) ---
	ErrNodeLocked = errors.New("cannot start locked roadmap node")
)
