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
}
