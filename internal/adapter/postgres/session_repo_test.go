package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	"github.com/leftmy/planetarium-server/internal/domain"
)

// seedUser inserts a user so the rows under test have something to reference.
func seedUser(t *testing.T, db *postgres.DB, email string) domain.User {
	t.Helper()

	user := newUser(t, email)

	if err := postgres.NewUserRepo(db).Create(context.Background(), user); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return user
}

func newID(t *testing.T) uuid.UUID {
	t.Helper()

	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}

	return id
}

func TestUserIdentityRepoCreateAndByProvider(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewUserIdentityRepo(db)

	user := seedUser(t, db, "identity@example.com")

	want := domain.UserIdentity{
		ID:             newID(t),
		UserID:         user.ID,
		Provider:       domain.ProviderLocal,
		ProviderUserID: user.ID.String(),
	}

	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.ByProvider(ctx, want.Provider, want.ProviderUserID)
	if err != nil {
		t.Fatalf("by provider: %v", err)
	}

	if got.ID != want.ID || got.UserID != want.UserID {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUserIdentityRepoByProviderNotFound(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserIdentityRepo(postgres.NewTestDB(t))

	_, err := repo.ByProvider(ctx, domain.ProviderGoogle, "no-such-account")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
	}
}

// UNIQUE (provider, provider_user_id) is what stops one Google account from
// being attached to two local users.
func TestUserIdentityRepoRejectsDuplicateProviderAccount(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewUserIdentityRepo(db)

	first := seedUser(t, db, "first@example.com")
	second := seedUser(t, db, "second@example.com")

	shared := "google-user-42"

	if err := repo.Create(ctx, domain.UserIdentity{
		ID: newID(t), UserID: first.ID, Provider: domain.ProviderGoogle, ProviderUserID: shared,
	}); err != nil {
		t.Fatalf("create first: %v", err)
	}

	err := repo.Create(ctx, domain.UserIdentity{
		ID: newID(t), UserID: second.ID, Provider: domain.ProviderGoogle, ProviderUserID: shared,
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want %v", err, domain.ErrAlreadyExists)
	}
}

// newRefreshToken stores one token for a user and returns it.
func newRefreshToken(t *testing.T, repo *postgres.RefreshTokenRepo, userID uuid.UUID, hash string) domain.RefreshToken {
	t.Helper()

	token := domain.RefreshToken{
		ID:        newID(t),
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := repo.Create(context.Background(), token); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	return token
}

func TestRefreshTokenRepoCreateAndByHash(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewRefreshTokenRepo(db)

	user := seedUser(t, db, "session@example.com")
	want := newRefreshToken(t, repo, user.ID, "hash-of-the-first-token")

	got, err := repo.ByHash(ctx, want.TokenHash)
	if err != nil {
		t.Fatalf("by hash: %v", err)
	}

	if got.ID != want.ID || got.UserID != user.ID {
		t.Errorf("got %+v, want id %v user %v", got, want.ID, user.ID)
	}

	if got.Revoked() {
		t.Error("a freshly created token is already revoked")
	}

	if got.Expired(time.Now()) {
		t.Error("a freshly created token is already expired")
	}
}

func TestRefreshTokenRepoByHashNotFound(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewRefreshTokenRepo(postgres.NewTestDB(t))

	_, err := repo.ByHash(ctx, "a hash nobody ever stored")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
	}
}

// Rotation: the old token is revoked and points at its successor, and ByHash
// still finds it. A repository that deleted the row instead would lose the
// evidence that reuse detection depends on.
func TestRefreshTokenRepoRevokeRecordsSuccessor(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewRefreshTokenRepo(db)

	user := seedUser(t, db, "rotate@example.com")
	old := newRefreshToken(t, repo, user.ID, "hash-old")
	next := newRefreshToken(t, repo, user.ID, "hash-new")

	if err := repo.Revoke(ctx, old.ID, next.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, err := repo.ByHash(ctx, old.TokenHash)
	if err != nil {
		t.Fatalf("by hash after revoke: %v", err)
	}

	if !got.Revoked() {
		t.Fatal("token is not marked revoked")
	}

	if got.ReplacedBy == nil || *got.ReplacedBy != next.ID {
		t.Errorf("replaced_by = %v, want %v", got.ReplacedBy, next.ID)
	}
}

// Revoking twice must fail rather than silently succeed: that zero-row result is
// how two simultaneous refreshes of the same token are told apart, and only one
// of them may win.
func TestRefreshTokenRepoRevokeIsNotRepeatable(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewRefreshTokenRepo(db)

	user := seedUser(t, db, "double-revoke@example.com")
	old := newRefreshToken(t, repo, user.ID, "hash-old")
	next := newRefreshToken(t, repo, user.ID, "hash-new")

	if err := repo.Revoke(ctx, old.ID, next.ID); err != nil {
		t.Fatalf("first revoke: %v", err)
	}

	err := repo.Revoke(ctx, old.ID, next.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second revoke: err = %v, want %v", err, domain.ErrNotFound)
	}
}

// The theft response: presenting an already revoked token must be able to take
// down every live session of that user, not just the one being refreshed.
func TestRefreshTokenRepoRevokeChain(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewRefreshTokenRepo(db)

	victim := seedUser(t, db, "victim@example.com")
	bystander := seedUser(t, db, "bystander@example.com")

	victimTokens := []domain.RefreshToken{
		newRefreshToken(t, repo, victim.ID, "victim-phone"),
		newRefreshToken(t, repo, victim.ID, "victim-laptop"),
		newRefreshToken(t, repo, victim.ID, "victim-tablet"),
	}

	other := newRefreshToken(t, repo, bystander.ID, "bystander-laptop")

	if err := repo.RevokeChain(ctx, victim.ID); err != nil {
		t.Fatalf("revoke chain: %v", err)
	}

	for _, want := range victimTokens {
		got, err := repo.ByHash(ctx, want.TokenHash)
		if err != nil {
			t.Fatalf("by hash %q: %v", want.TokenHash, err)
		}

		if !got.Revoked() {
			t.Errorf("token %q survived the chain revocation", want.TokenHash)
		}
	}

	// Another user's sessions must be untouched.
	got, err := repo.ByHash(ctx, other.TokenHash)
	if err != nil {
		t.Fatalf("by hash for the other user: %v", err)
	}

	if got.Revoked() {
		t.Error("revoking one user's chain also revoked another user's session")
	}
}
