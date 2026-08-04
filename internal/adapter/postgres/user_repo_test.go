package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	"github.com/leftmy/planetarium-server/internal/domain"
)

// newUser builds a valid user with a fresh ID, so a test only has to state the
// part it actually cares about.
func newUser(t *testing.T, email string) domain.User {
	t.Helper()

	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}

	addr, err := domain.NewEmail(email)
	if err != nil {
		t.Fatalf("new email %q: %v", email, err)
	}

	return domain.User{
		ID:           id,
		Name:         "Test User",
		Email:        addr,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
	}
}

func TestUserRepoCreateAndByEmail(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(postgres.NewTestDB(t))

	want := newUser(t, "ada@example.com")

	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.ByEmail(ctx, want.Email)
	if err != nil {
		t.Fatalf("by email: %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("id = %v, want %v", got.ID, want.ID)
	}

	if got.Email != want.Email {
		t.Errorf("email = %q, want %q", got.Email, want.Email)
	}

	if got.PasswordHash != want.PasswordHash {
		t.Errorf("password hash = %q, want %q", got.PasswordHash, want.PasswordHash)
	}
}

// The email column is CITEXT, so an address stored in one case must be found in
// another. Getting this wrong lets the same person register twice.
func TestUserRepoByEmailIsCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(postgres.NewTestDB(t))

	user := newUser(t, "grace@example.com")

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := repo.ByEmail(ctx, domain.Email("GRACE@Example.COM")); err != nil {
		t.Fatalf("by email with different case: %v", err)
	}
}

func TestUserRepoByEmailNotFound(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(postgres.NewTestDB(t))

	_, err := repo.ByEmail(ctx, domain.Email("nobody@example.com"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
	}
}

// A duplicate address must surface as a domain error rather than a Postgres one:
// the registration usecase relies on the constraint instead of a preceding
// SELECT, which is what makes two concurrent registrations safe.
func TestUserRepoCreateDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(postgres.NewTestDB(t))

	first := newUser(t, "linus@example.com")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	second := newUser(t, "linus@example.com")

	err := repo.Create(ctx, second)
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want %v", err, domain.ErrAlreadyExists)
	}
}

// WithinTx must roll the whole unit back, not just the statement that failed.
// The registration usecase writes users and user_identities together and depends
// on this.
func TestWithinTxRollsBack(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)
	repo := postgres.NewUserRepo(db)

	user := newUser(t, "rollback@example.com")
	sentinel := errors.New("usecase failed after insert")

	err := db.WithinTx(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, user); err != nil {
			return err
		}

		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}

	if _, err := repo.ByEmail(ctx, user.Email); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("user survived the rollback: err = %v, want %v", err, domain.ErrNotFound)
	}
}

// Each NewTestDB call must hand back an empty database; otherwise tests start
// depending on the order they run in.
func TestNewTestDBStartsClean(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepo(postgres.NewTestDB(t))

	// The same address other tests in this package insert.
	if _, err := repo.ByEmail(ctx, domain.Email("ada@example.com")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("leftover row from another test: err = %v", err)
	}
}
