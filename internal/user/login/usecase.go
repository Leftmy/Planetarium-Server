package login

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/pkg/hash"
	"github.com/leftmy/planetarium-server/pkg/token"
)

type userRepo interface {
	ByEmail(ctx context.Context, email domain.Email) (domain.User, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

type refreshTokenRepo interface {
	Create(ctx context.Context, t domain.RefreshToken) error
	ByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, error)
	Revoke(ctx context.Context, id, replacedBy uuid.UUID) error
	RevokeChain(ctx context.Context, userID uuid.UUID) error
}

type transactor interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

type Usecase struct {
	tx       transactor
	users    userRepo
	sessions refreshTokenRepo
	tokens   *token.Manager

	// now is time.Now outside tests, which need to present an expired token
	// without waiting a month for one.
	now func() time.Time
}

func NewUsecase(tx transactor, users userRepo, sessions refreshTokenRepo, tokens *token.Manager) *Usecase {
	return &Usecase{tx: tx, users: users, sessions: sessions, tokens: tokens, now: time.Now}
}

// Login exchanges an email and password for a token pair.
//
// Every failure returns domain.ErrInvalidCredentials, whether the address is
// unknown, the password is wrong, or the account has no password because it was
// created through OAuth. Distinguishing them would turn the login form into a
// tool for checking who is registered.
func (u *Usecase) Login(ctx context.Context, req Request) (Response, error) {
	email := domain.Email(strings.ToLower(strings.TrimSpace(req.Email)))

	user, err := u.users.ByEmail(ctx, email)

	switch {
	case errors.Is(err, domain.ErrNotFound):
		// The password is still verified, against a hash nobody owns. Skipping
		// this would make a missing account answer noticeably faster than a wrong
		// password, which gives away exactly what the identical error message is
		// there to hide.
		if _, vErr := hash.Verify(req.Password, hash.DummyHash); vErr != nil {
			return Response{}, fmt.Errorf("verify dummy hash: %w", vErr)
		}

		return Response{}, domain.ErrInvalidCredentials

	case err != nil:
		return Response{}, err
	}

	// An OAuth-only account has no password hash. Verifying an empty string
	// against it would be a malformed-hash error, so the case is handled first —
	// but with the same answer as any other failure.
	if user.PasswordHash == "" {
		if _, vErr := hash.Verify(req.Password, hash.DummyHash); vErr != nil {
			return Response{}, fmt.Errorf("verify dummy hash: %w", vErr)
		}

		return Response{}, domain.ErrInvalidCredentials
	}

	ok, err := hash.Verify(req.Password, user.PasswordHash)
	if err != nil {
		// A stored hash that cannot be read is corrupt data, not a wrong
		// password. Reporting it as invalid credentials would hide the problem
		// behind a plausible-looking 401 forever.
		return Response{}, fmt.Errorf("verify password: %w", err)
	}

	if !ok {
		return Response{}, domain.ErrInvalidCredentials
	}

	return u.issue(ctx, user)
}

// Refresh exchanges a refresh token for a new pair and retires the old one.
func (u *Usecase) Refresh(ctx context.Context, req RefreshRequest) (Response, error) {
	if req.RefreshToken == "" {
		return Response{}, domain.ErrInvalidRefreshToken
	}

	stored, err := u.sessions.ByHash(ctx, token.HashRefresh(req.RefreshToken))

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return Response{}, domain.ErrInvalidRefreshToken
	case err != nil:
		return Response{}, err
	}

	// A token presented after it was already exchanged means a copy of it exists
	// somewhere it should not. The session it was exchanged for is therefore
	// suspect too, so the whole chain goes rather than just this request.
	if stored.Revoked() {
		if err := u.sessions.RevokeChain(ctx, stored.UserID); err != nil {
			return Response{}, fmt.Errorf("revoke chain after token reuse: %w", err)
		}

		return Response{}, domain.ErrInvalidRefreshToken
	}

	if stored.Expired(u.now()) {
		return Response{}, domain.ErrInvalidRefreshToken
	}

	user, err := u.users.ByID(ctx, stored.UserID)

	switch {
	case errors.Is(err, domain.ErrNotFound):
		// The account was deleted while the session was alive.
		return Response{}, domain.ErrInvalidRefreshToken
	case err != nil:
		return Response{}, err
	}

	return u.rotate(ctx, user, stored)
}

// issue mints a fresh pair and records the refresh half.
func (u *Usecase) issue(ctx context.Context, user domain.User) (Response, error) {
	resp, _, err := u.newPair(ctx, user)

	return resp, err
}

// rotate issues a new pair and retires the presented token in one transaction,
// so a failure cannot leave the user with a revoked token and no replacement.
func (u *Usecase) rotate(ctx context.Context, user domain.User, old domain.RefreshToken) (Response, error) {
	var resp Response

	err := u.tx.WithinTx(ctx, func(ctx context.Context) error {
		newResp, newID, err := u.newPair(ctx, user)
		if err != nil {
			return err
		}

		// Revoke refuses to act on an already revoked row, so if two requests
		// present the same token at once, only one of them gets this far and the
		// other rolls back.
		if err := u.sessions.Revoke(ctx, old.ID, newID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.ErrInvalidRefreshToken
			}

			return err
		}

		resp = newResp

		return nil
	})
	if err != nil {
		return Response{}, err
	}

	return resp, nil
}

// newPair signs an access token, stores a refresh token and builds the response.
// It returns the stored token's ID so a rotation can point the old row at it.
func (u *Usecase) newPair(ctx context.Context, user domain.User) (Response, uuid.UUID, error) {
	access, accessExpires, err := u.tokens.NewAccess(user.ID)
	if err != nil {
		return Response{}, uuid.Nil, fmt.Errorf("new access token: %w", err)
	}

	plain, hashed, refreshExpires, err := u.tokens.NewRefresh()
	if err != nil {
		return Response{}, uuid.Nil, fmt.Errorf("new refresh token: %w", err)
	}

	id, err := domain.NewID()
	if err != nil {
		return Response{}, uuid.Nil, fmt.Errorf("new refresh token id: %w", err)
	}

	err = u.sessions.Create(ctx, domain.RefreshToken{
		ID:        id,
		UserID:    user.ID,
		TokenHash: hashed,
		ExpiresAt: refreshExpires,
	})
	if err != nil {
		return Response{}, uuid.Nil, err
	}

	return Response{
		AccessToken:  access,
		RefreshToken: plain,
		TokenType:    "Bearer",
		ExpiresIn:    int(time.Until(accessExpires).Seconds()),
		User: User{
			ID:    user.ID,
			Email: string(user.Email),
			Name:  user.Name,
		},
	}, id, nil
}
