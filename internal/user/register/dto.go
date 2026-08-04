package register

import "github.com/google/uuid"

type Request struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Response is what a successful registration returns.
//
// It has no field for the password or its hash, and must not grow one: this
// struct is the last thing between the database row and the network.
type Response struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}
