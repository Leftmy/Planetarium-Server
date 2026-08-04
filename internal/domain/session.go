package domain

import (
	"time"

	"github.com/google/uuid"
)

// Identity providers a user_identities row may name. They match the CHECK
// constraint in the schema; adding one here without adding it there makes the
// insert fail at runtime.
const (
	ProviderLocal  = "local"
	ProviderGoogle = "google"
	ProviderGitHub = "github"
)

// UserIdentity is one way of signing in to an account. A password account has a
// single row with ProviderLocal; connecting Google later adds a second row
// rather than replacing the first, which is what lets both keep working.
type UserIdentity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       string
	ProviderUserID string

	// Email is what the provider reported, kept for linking an OAuth login to an
	// existing account later. Empty for ProviderLocal, where users.email is the
	// authoritative copy.
	Email string
}

// RefreshToken is one issued session token, stored as a hash.
//
// ReplacedBy implements rotation: when a token is exchanged, it is revoked and
// points at its successor. A token that is presented after being revoked means a
// copy of it exists somewhere it should not, and the right response is to revoke
// the user's whole chain rather than to refuse this one request.
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  string
	ReplacedBy *uuid.UUID
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

// Revoked reports whether the token has already been exchanged or invalidated.
func (t RefreshToken) Revoked() bool {
	return t.RevokedAt != nil
}

// Expired reports whether the token is past its lifetime at the given moment.
func (t RefreshToken) Expired(now time.Time) bool {
	return now.After(t.ExpiresAt)
}

// TokenPair is what a successful login or refresh hands back. The refresh token
// here is the plaintext — the only moment it exists outside the client, since
// the database stores only its hash.
type TokenPair struct {
	AccessToken     string
	RefreshToken    string
	AccessExpiresAt time.Time
}
