package register

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/pkg/hash"
)

const (
	minPasswordLength = 8

	// Passwords are capped in bytes rather than characters, and the limit is
	// about request size, not cryptography: argon2id has no input limit of its
	// own. 72 is bcrypt's limit, kept so that switching algorithms later could
	// never silently truncate an existing password.
	maxPasswordBytes = 72

	maxNameLength = 100
)

// userRepo and identityRepo are the slices' view of the storage they need.
// Narrow interfaces declared here rather than in the adapter mean this package
// depends on what it uses, and a test can substitute either half.
type userRepo interface {
	Create(ctx context.Context, u domain.User) error
}

type identityRepo interface {
	Create(ctx context.Context, identity domain.UserIdentity) error
}

// transactor is implemented by *postgres.DB. The usecase does not know what a
// transaction is beyond "everything in this function commits or none of it does".
type transactor interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

type Usecase struct {
	tx         transactor
	users      userRepo
	identities identityRepo
}

func NewUsecase(tx transactor, users userRepo, identities identityRepo) *Usecase {
	return &Usecase{tx: tx, users: users, identities: identities}
}

// Register creates the account and its local sign-in method.
//
// Both rows are written in one transaction: a user without an identity could
// never sign in, and would also block the address from being registered again.
func (u *Usecase) Register(ctx context.Context, req Request) (Response, error) {
	email, err := domain.NewEmail(strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return Response{}, err
	}

	name := strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxNameLength {
		return Response{}, domain.ErrInvalidName
	}

	if len(req.Password) < minPasswordLength {
		return Response{}, domain.ErrPasswordWeak
	}

	if len(req.Password) > maxPasswordBytes {
		return Response{}, domain.ErrPasswordLong
	}

	passwordHash, err := hash.Password(req.Password)
	if err != nil {
		return Response{}, fmt.Errorf("hash password: %w", err)
	}

	userID, err := domain.NewID()
	if err != nil {
		return Response{}, fmt.Errorf("new user id: %w", err)
	}

	identityID, err := domain.NewID()
	if err != nil {
		return Response{}, fmt.Errorf("new identity id: %w", err)
	}

	user := domain.User{
		ID:           userID,
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
	}

	err = u.tx.WithinTx(ctx, func(ctx context.Context) error {
		// A duplicate address surfaces here, from the unique index, rather than
		// from a SELECT beforehand. Checking first would leave a window in which
		// two concurrent registrations both find the address free.
		if err := u.users.Create(ctx, user); err != nil {
			return err
		}

		return u.identities.Create(ctx, domain.UserIdentity{
			ID:             identityID,
			UserID:         user.ID,
			Provider:       domain.ProviderLocal,
			ProviderUserID: user.ID.String(),
		})
	})
	if err != nil {
		return Response{}, err
	}

	return Response{
		ID:    user.ID,
		Email: string(user.Email),
		Name:  user.Name,
	}, nil
}
