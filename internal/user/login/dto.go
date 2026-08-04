package login

import "github.com/google/uuid"

// Request is the login body. Refresh has its own type below, so neither endpoint
// accepts fields it does not use.
type Request struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest carries the token being exchanged. It lives in the body rather
// than a cookie, as agreed in docs/api-contract-draft.md: that keeps web and any
// future mobile client on one path, at the cost of the token being reachable
// from JavaScript.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// User is the account summary returned alongside the tokens. It deliberately has
// no password field of any kind.
type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}

// Response is returned by both /login and /refresh, so a client parses one shape
// for both.
type Response struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`

	// ExpiresIn is seconds remaining on the access token, so a client does not
	// have to decode the JWT just to learn when to refresh.
	ExpiresIn int `json:"expires_in"`

	User User `json:"user"`
}
