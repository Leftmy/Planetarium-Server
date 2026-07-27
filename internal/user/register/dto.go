package register

import "github.com/google/uuid"

type Request struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Response struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}
