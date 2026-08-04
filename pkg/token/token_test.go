package token_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/leftmy/planetarium-server/pkg/token"
)

const (
	secret     = "test-secret-not-used-anywhere-else"
	accessTTL  = 15 * time.Minute
	refreshTTL = 30 * 24 * time.Hour
)

func newManager(t *testing.T) *token.Manager {
	t.Helper()

	return token.NewManager(secret, accessTTL, refreshTTL)
}

func newUserID(t *testing.T) uuid.UUID {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}

	return id
}

func TestAccessRoundTrip(t *testing.T) {
	m := newManager(t)
	want := newUserID(t)

	raw, expires, err := m.NewAccess(want)
	if err != nil {
		t.Fatalf("new access: %v", err)
	}

	if d := time.Until(expires); d > accessTTL || d < accessTTL-time.Minute {
		t.Errorf("expires in %v, want about %v", d, accessTTL)
	}

	got, err := m.ParseAccess(raw)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}

	if got != want {
		t.Errorf("subject = %v, want %v", got, want)
	}
}

// Two tokens for the same user issued in the same second must still differ, or
// a refresh can hand back the exact token it was meant to replace.
func TestAccessTokensAreUnique(t *testing.T) {
	m := newManager(t)
	user := newUserID(t)

	first, _, err := m.NewAccess(user)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, _, err := m.NewAccess(user)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first == second {
		t.Fatal("two access tokens for the same user came out identical")
	}

	// Both must still name the same user.
	for i, raw := range []string{first, second} {
		got, err := m.ParseAccess(raw)
		if err != nil {
			t.Fatalf("parse %d: %v", i, err)
		}

		if got != user {
			t.Errorf("token %d subject = %v, want %v", i, got, user)
		}
	}
}

// An expired token must be refused. This is the whole reason the access token is
// short-lived, so it is worth proving rather than assuming.
func TestParseAccessRejectsExpiredToken(t *testing.T) {
	m := newManager(t)

	raw, _, err := m.NewAccess(newUserID(t))
	if err != nil {
		t.Fatalf("new access: %v", err)
	}

	// A second manager that believes it is an hour later. Same secret, so the
	// signature is still good and only the expiry can fail the token.
	future := token.NewManager(secret, accessTTL, refreshTTL)
	token.SetNow(future, func() time.Time { return time.Now().Add(time.Hour) })

	if _, err := future.ParseAccess(raw); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("err = %v, want %v", err, token.ErrInvalidToken)
	}
}

func TestParseAccessRejectsTamperedPayload(t *testing.T) {
	m := newManager(t)

	raw, _, err := m.NewAccess(newUserID(t))
	if err != nil {
		t.Fatalf("new access: %v", err)
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}

	// Swap one character of the payload: the claims change, the signature does not.
	payload := []byte(parts[1])
	if payload[0] == 'A' {
		payload[0] = 'B'
	} else {
		payload[0] = 'A'
	}

	tampered := parts[0] + "." + string(payload) + "." + parts[2]

	if _, err := m.ParseAccess(tampered); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("err = %v, want %v", err, token.ErrInvalidToken)
	}
}

func TestParseAccessRejectsForeignSecret(t *testing.T) {
	issuer := token.NewManager("some other service's secret", accessTTL, refreshTTL)

	raw, _, err := issuer.NewAccess(newUserID(t))
	if err != nil {
		t.Fatalf("new access: %v", err)
	}

	if _, err := newManager(t).ParseAccess(raw); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("err = %v, want %v", err, token.ErrInvalidToken)
	}
}

// "alg": "none" is the classic JWT attack: a token with no signature at all,
// accepted by libraries that trust the header. jwt.WithValidMethods is what
// stops it, and this test is here to notice if that option is ever dropped.
func TestParseAccessRejectsUnsignedToken(t *testing.T) {
	m := newManager(t)

	// {"alg":"none","typ":"JWT"} . {"sub":"..."} . <empty signature>
	unsigned := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiIwMTk4ZjNhMC0wMDAwLTcwMDAtODAwMC0wMDAwMDAwMDAwMDAifQ."

	if _, err := m.ParseAccess(unsigned); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("err = %v, want %v", err, token.ErrInvalidToken)
	}
}

func TestParseAccessRejectsGarbage(t *testing.T) {
	m := newManager(t)

	for _, raw := range []string{"", "not a token", "a.b.c"} {
		if _, err := m.ParseAccess(raw); !errors.Is(err, token.ErrInvalidToken) {
			t.Errorf("ParseAccess(%q): err = %v, want %v", raw, err, token.ErrInvalidToken)
		}
	}
}

func TestNewRefreshIsRandomAndHashed(t *testing.T) {
	m := newManager(t)

	first, firstHash, expires, err := m.NewRefresh()
	if err != nil {
		t.Fatalf("new refresh: %v", err)
	}

	second, secondHash, _, err := m.NewRefresh()
	if err != nil {
		t.Fatalf("new refresh: %v", err)
	}

	if first == second {
		t.Error("two refresh tokens came out identical")
	}

	if firstHash == secondHash {
		t.Error("two refresh tokens hashed to the same value")
	}

	if strings.Contains(firstHash, first) {
		t.Error("the stored hash contains the plaintext token")
	}

	if got := token.HashRefresh(first); got != firstHash {
		t.Errorf("HashRefresh is not stable: %q != %q", got, firstHash)
	}

	if d := time.Until(expires); d > refreshTTL || d < refreshTTL-time.Minute {
		t.Errorf("expires in %v, want about %v", d, refreshTTL)
	}
}
