package domain

import (
	"regexp"

	"github.com/google/uuid"
)

type Email string

func NewEmail(raw string) (Email, error) {
	re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	if !re.MatchString(raw) {
		return "", ErrInvalidEmail
	}
	return Email(raw), nil
}

type User struct {
	ID    uuid.UUID
	Name  string
	Email Email

	// PasswordHash is an argon2id string with its parameters embedded, so the
	// cost can be raised later without invalidating existing passwords. Empty
	// for accounts that only ever signed in through OAuth.
	PasswordHash string
}
