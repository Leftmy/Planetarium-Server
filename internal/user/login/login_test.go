package login_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/internal/user/login"
	"github.com/leftmy/planetarium-server/internal/user/register"
	"github.com/leftmy/planetarium-server/pkg/token"
)

const (
	testEmail    = "ada@example.com"
	testPassword = "correct horse battery staple"
	testSecret   = "test-secret-not-used-anywhere-else"
)

type fixture struct {
	db       *postgres.DB
	users    *postgres.UserRepo
	sessions *postgres.RefreshTokenRepo
	uc       *login.Usecase
	tokens   *token.Manager
}

// newFixture wires the real slice against a real database and registers one
// account, since a login test without a registration is testing nothing.
func newFixture(t *testing.T) fixture {
	t.Helper()

	db := postgres.NewTestDB(t)
	users := postgres.NewUserRepo(db)
	sessions := postgres.NewRefreshTokenRepo(db)
	tokens := token.NewManager(testSecret, 15*time.Minute, 30*24*time.Hour)

	reg := register.NewUsecase(db, users, postgres.NewUserIdentityRepo(db))

	_, err := reg.Register(context.Background(), register.Request{
		Name: "Ada Lovelace", Email: testEmail, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("seed registration: %v", err)
	}

	return fixture{
		db:       db,
		users:    users,
		sessions: sessions,
		uc:       login.NewUsecase(db, users, sessions, tokens),
		tokens:   tokens,
	}
}

func TestLoginSucceeds(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	resp, err := f.uc.Login(ctx, login.Request{Email: testEmail, Password: testPassword})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("login returned an empty token")
	}

	if resp.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", resp.TokenType)
	}

	if resp.ExpiresIn <= 0 || resp.ExpiresIn > 900 {
		t.Errorf("expires_in = %d, want 0 < n <= 900", resp.ExpiresIn)
	}

	if resp.User.Email != testEmail {
		t.Errorf("user.email = %q, want %q", resp.User.Email, testEmail)
	}

	// The access token must actually be ours and name the user who logged in.
	subject, err := f.tokens.ParseAccess(resp.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if subject != resp.User.ID {
		t.Errorf("token subject = %v, want %v", subject, resp.User.ID)
	}

	// The refresh token must be stored as a hash, never as itself.
	if _, err := f.sessions.ByHash(ctx, token.HashRefresh(resp.RefreshToken)); err != nil {
		t.Fatalf("refresh token was not stored: %v", err)
	}

	if _, err := f.sessions.ByHash(ctx, resp.RefreshToken); !errors.Is(err, domain.ErrNotFound) {
		t.Error("the refresh token is stored in plaintext")
	}
}

// The three ways a login can fail must be indistinguishable from outside.
// Different errors would let anyone use the login form to find out which
// addresses have accounts.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	// An OAuth-style account: a row with no password hash at all.
	oauthUser := domain.User{Name: "OAuth Only", Email: domain.Email("oauth@example.com")}

	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}

	oauthUser.ID = id

	if err := f.users.Create(ctx, oauthUser); err != nil {
		t.Fatalf("seed oauth user: %v", err)
	}

	tests := []struct {
		name string
		req  login.Request
	}{
		{"unknown email", login.Request{Email: "nobody@example.com", Password: testPassword}},
		{"wrong password", login.Request{Email: testEmail, Password: "not the password"}},
		{"empty password", login.Request{Email: testEmail, Password: ""}},
		{"account without a password", login.Request{Email: "oauth@example.com", Password: testPassword}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.uc.Login(ctx, tt.req)
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want %v", err, domain.ErrInvalidCredentials)
			}
		})
	}
}

func TestLoginIsCaseInsensitiveOnEmail(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	if _, err := f.uc.Login(ctx, login.Request{Email: "  ADA@Example.COM ", Password: testPassword}); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestRefreshRotatesTheTokenPair(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	first, err := f.uc.Login(ctx, login.Request{Email: testEmail, Password: testPassword})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	second, err := f.uc.Refresh(ctx, login.RefreshRequest{RefreshToken: first.RefreshToken})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh returned the same token, so nothing rotated")
	}

	if second.User.ID != first.User.ID {
		t.Errorf("user changed across refresh: %v then %v", first.User.ID, second.User.ID)
	}

	// The old row must point at its successor, which is what makes a chain a chain.
	old, err := f.sessions.ByHash(ctx, token.HashRefresh(first.RefreshToken))
	if err != nil {
		t.Fatalf("load old token: %v", err)
	}

	if !old.Revoked() {
		t.Error("the exchanged token is still live")
	}

	next, err := f.sessions.ByHash(ctx, token.HashRefresh(second.RefreshToken))
	if err != nil {
		t.Fatalf("load new token: %v", err)
	}

	if old.ReplacedBy == nil || *old.ReplacedBy != next.ID {
		t.Errorf("replaced_by = %v, want %v", old.ReplacedBy, next.ID)
	}
}

func TestRefreshRejectsUnknownToken(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	for _, raw := range []string{"", "not-a-token-anyone-issued"} {
		_, err := f.uc.Refresh(ctx, login.RefreshRequest{RefreshToken: raw})
		if !errors.Is(err, domain.ErrInvalidRefreshToken) {
			t.Errorf("Refresh(%q): err = %v, want %v", raw, err, domain.ErrInvalidRefreshToken)
		}
	}
}

// The theft scenario, end to end. Presenting an already exchanged token means a
// copy of it is in someone else's hands, so the session it was exchanged for is
// suspect too — and everything the user has open must go, not just this request.
func TestRefreshReuseRevokesTheWholeChain(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	first, err := f.uc.Login(ctx, login.Request{Email: testEmail, Password: testPassword})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	second, err := f.uc.Refresh(ctx, login.RefreshRequest{RefreshToken: first.RefreshToken})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// The thief replays the token that was already used.
	if _, err := f.uc.Refresh(ctx, login.RefreshRequest{RefreshToken: first.RefreshToken}); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("replay: err = %v, want %v", err, domain.ErrInvalidRefreshToken)
	}

	// And the legitimate session, issued a moment ago, is gone too.
	if _, err := f.uc.Refresh(ctx, login.RefreshRequest{RefreshToken: second.RefreshToken}); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("the live session survived a detected reuse: err = %v", err)
	}

	live, err := f.sessions.ByHash(ctx, token.HashRefresh(second.RefreshToken))
	if err != nil {
		t.Fatalf("load the second token: %v", err)
	}

	if !live.Revoked() {
		t.Error("the second token is still marked live in the database")
	}
}

func TestRefreshRejectsExpiredToken(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	users := postgres.NewUserRepo(db)
	sessions := postgres.NewRefreshTokenRepo(db)

	// A manager whose refresh tokens were dead on arrival.
	expiring := token.NewManager(testSecret, 15*time.Minute, -time.Hour)
	uc := login.NewUsecase(db, users, sessions, expiring)

	reg := register.NewUsecase(db, users, postgres.NewUserIdentityRepo(db))

	if _, err := reg.Register(ctx, register.Request{
		Name: "Ada", Email: testEmail, Password: testPassword,
	}); err != nil {
		t.Fatalf("seed registration: %v", err)
	}

	resp, err := uc.Login(ctx, login.Request{Email: testEmail, Password: testPassword})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := uc.Refresh(ctx, login.RefreshRequest{RefreshToken: resp.RefreshToken}); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("err = %v, want %v", err, domain.ErrInvalidRefreshToken)
	}
}
