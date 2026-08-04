// Package token issues and checks the two credentials a session is made of.
//
// They are deliberately different things. The access token is a signed JWT: it
// is short-lived and verified without touching the database, which is what makes
// it cheap on every request. The refresh token is a random string stored in the
// database, because the whole point of it is that it can be revoked — something
// a signed token cannot be without a table to look it up in anyway.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken covers every reason an access token was not accepted:
// expired, tampered with, signed by someone else, or simply not a JWT. The
// reasons are deliberately not distinguished — a caller cannot act on the
// difference, and telling an attacker which part failed is a gift.
var ErrInvalidToken = errors.New("token: invalid access token")

// Manager issues both kinds of token. One instance is built at startup from
// config and shared; it holds no per-request state.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration

	// now is time.Now except in tests, which need to look at an expired token
	// without waiting fifteen minutes for one.
	now func() time.Time
}

func NewManager(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

func (m *Manager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// NewAccess signs a token for userID and reports when it stops being valid, so
// the caller can put expires_in in the response without parsing the token back.
func (m *Manager) NewAccess(userID uuid.UUID) (string, time.Time, error) {
	issued := m.now()
	expires := issued.Add(m.accessTTL)

	// ID makes every token distinct. Without it two tokens minted for the same
	// user in the same second are byte-identical, which is confusing to debug and
	// leaves nothing to name a single token by if access tokens ever need to be
	// blocklisted.
	jti, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token: new token id: %w", err)
	}

	claims := jwt.RegisteredClaims{
		ID:        jti.String(),
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(issued),
		ExpiresAt: jwt.NewNumericDate(expires),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token: sign access token: %w", err)
	}

	return signed, expires, nil
}

// ParseAccess checks the signature and expiry and returns the user the token
// belongs to.
func (m *Manager) ParseAccess(raw string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims

	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		// Without this check a token could arrive with alg "none", or signed
		// with an algorithm whose verification takes the secret as a public key.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}

		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: subject %q is not a uuid", ErrInvalidToken, claims.Subject)
	}

	return id, nil
}

// NewRefresh returns the token to hand to the client and the hash to store,
// alongside its expiry. The plaintext is never written down: a database dump
// must not hand over live sessions.
func (m *Manager) NewRefresh() (plain, hashed string, expires time.Time, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", time.Time{}, fmt.Errorf("token: read random bytes: %w", err)
	}

	plain = base64.RawURLEncoding.EncodeToString(raw)

	return plain, HashRefresh(plain), m.now().Add(m.refreshTTL), nil
}

// HashRefresh is what turns a presented token into the value stored in
// refresh_tokens.token_hash.
//
// SHA-256 rather than argon2: a password is short and guessable and needs a slow
// hash, while this token is 256 random bits, where brute force is not a threat
// that exists. A slow hash here would only add hundreds of milliseconds to every
// session refresh.
func HashRefresh(plain string) string {
	sum := sha256.Sum256([]byte(plain))

	return hex.EncodeToString(sum[:])
}
