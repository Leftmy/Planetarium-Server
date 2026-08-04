package register_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	"github.com/leftmy/planetarium-server/internal/domain"
	"github.com/leftmy/planetarium-server/internal/user/register"
	"github.com/leftmy/planetarium-server/pkg/hash"
)

func newUsecase(t *testing.T) (*register.Usecase, *postgres.DB) {
	t.Helper()

	db := postgres.NewTestDB(t)

	return register.NewUsecase(db, postgres.NewUserRepo(db), postgres.NewUserIdentityRepo(db)), db
}

func validRequest() register.Request {
	return register.Request{
		Name:     "Ada Lovelace",
		Email:    "ada@example.com",
		Password: "correct horse battery staple",
	}
}

func TestRegisterCreatesUserAndIdentity(t *testing.T) {
	ctx := context.Background()
	uc, db := newUsecase(t)

	resp, err := uc.Register(ctx, validRequest())
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if resp.Email != "ada@example.com" || resp.Name != "Ada Lovelace" {
		t.Errorf("response = %+v", resp)
	}

	user, err := postgres.NewUserRepo(db).ByEmail(ctx, domain.Email("ada@example.com"))
	if err != nil {
		t.Fatalf("by email: %v", err)
	}

	// The password must be stored as a hash of itself and nothing else.
	if user.PasswordHash == validRequest().Password {
		t.Fatal("the password was stored in plaintext")
	}

	ok, err := hash.Verify(validRequest().Password, user.PasswordHash)
	if err != nil {
		t.Fatalf("verify stored hash: %v", err)
	}

	if !ok {
		t.Error("the stored hash does not verify the password it was made from")
	}

	// The local sign-in method must exist, otherwise the account could never log in.
	identity, err := postgres.NewUserIdentityRepo(db).ByProvider(ctx, domain.ProviderLocal, user.ID.String())
	if err != nil {
		t.Fatalf("by provider: %v", err)
	}

	if identity.UserID != user.ID {
		t.Errorf("identity belongs to %v, want %v", identity.UserID, user.ID)
	}
}

func TestRegisterNormalisesEmail(t *testing.T) {
	ctx := context.Background()
	uc, _ := newUsecase(t)

	req := validRequest()
	req.Email = "  Ada@Example.COM  "

	resp, err := uc.Register(ctx, req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if resp.Email != "ada@example.com" {
		t.Errorf("email = %q, want %q", resp.Email, "ada@example.com")
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	uc, _ := newUsecase(t)

	if _, err := uc.Register(ctx, validRequest()); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Same address in a different case must still collide: the column is CITEXT.
	second := validRequest()
	second.Email = "ADA@example.com"

	_, err := uc.Register(ctx, second)
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want %v", err, domain.ErrAlreadyExists)
	}
}

func TestRegisterValidation(t *testing.T) {
	ctx := context.Background()
	uc, _ := newUsecase(t)

	tests := []struct {
		name   string
		mutate func(*register.Request)
		want   error
	}{
		{"no @", func(r *register.Request) { r.Email = "not-an-email" }, domain.ErrInvalidEmail},
		{"empty email", func(r *register.Request) { r.Email = "" }, domain.ErrInvalidEmail},
		{"short password", func(r *register.Request) { r.Password = "1234567" }, domain.ErrPasswordWeak},
		{"empty password", func(r *register.Request) { r.Password = "" }, domain.ErrPasswordWeak},
		{"password over 72 bytes", func(r *register.Request) { r.Password = strings.Repeat("a", 73) }, domain.ErrPasswordLong},
		{"empty name", func(r *register.Request) { r.Name = "   " }, domain.ErrInvalidName},
		{"name over 100 chars", func(r *register.Request) { r.Name = strings.Repeat("я", 101) }, domain.ErrInvalidName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRequest()
			tt.mutate(&req)

			if _, err := uc.Register(ctx, req); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// failingIdentityRepo stands in for the second insert going wrong.
type failingIdentityRepo struct {
	err error
}

func (f failingIdentityRepo) Create(context.Context, domain.UserIdentity) error { return f.err }

// If the identity insert fails, the user must not survive either: an account
// with no sign-in method could never be used, and would hold its email address
// hostage against a second registration attempt.
func TestRegisterRollsBackWhenIdentityFails(t *testing.T) {
	ctx := context.Background()
	db := postgres.NewTestDB(t)

	boom := errors.New("identity storage is down")
	uc := register.NewUsecase(db, postgres.NewUserRepo(db), failingIdentityRepo{err: boom})

	if _, err := uc.Register(ctx, validRequest()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}

	_, err := postgres.NewUserRepo(db).ByEmail(ctx, domain.Email("ada@example.com"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("the user survived the rollback: err = %v", err)
	}
}

// stubUsecase lets the handler tests run without a database.
type stubUsecase struct {
	resp register.Response
	err  error
}

func (s stubUsecase) Register(context.Context, register.Request) (register.Response, error) {
	return s.resp, s.err
}

func TestHandlerStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"success", `{"name":"Ada","email":"ada@example.com","password":"correct horse"}`, nil, http.StatusCreated, ""},
		{"malformed json", `{`, nil, http.StatusBadRequest, "invalid_request"},
		{"duplicate email", `{}`, domain.ErrAlreadyExists, http.StatusConflict, "email_already_exists"},
		{"invalid email", `{}`, domain.ErrInvalidEmail, http.StatusBadRequest, "invalid_email"},
		{"weak password", `{}`, domain.ErrPasswordWeak, http.StatusBadRequest, "password_too_weak"},
		{"unexpected failure", `{}`, errors.New("database on fire"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := register.NewHandler(stubUsecase{
				resp: register.Response{Email: "ada@example.com", Name: "Ada"},
				err:  tt.err,
			})

			rec := httptest.NewRecorder()
			h.HTTPv1(rec, httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(tt.body)))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}

			if tt.wantCode == "" {
				return
			}

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body %s: %v", rec.Body, err)
			}

			if body.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tt.wantCode)
			}

			if body.Error.Message == "" {
				t.Error("error message is empty")
			}
		})
	}
}

// The response is the last thing between a database row and the network, so it
// is worth asserting on the bytes rather than on the struct.
func TestHandlerNeverLeaksPasswordFields(t *testing.T) {
	h := register.NewHandler(stubUsecase{
		resp: register.Response{Email: "ada@example.com", Name: "Ada"},
	})

	rec := httptest.NewRecorder()
	h.HTTPv1(rec, httptest.NewRequest(http.MethodPost, "/api/v1/register",
		strings.NewReader(`{"name":"Ada","email":"ada@example.com","password":"correct horse"}`)))

	body := strings.ToLower(rec.Body.String())

	for _, forbidden := range []string{"password", "hash", "argon2"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response contains %q: %s", forbidden, rec.Body)
		}
	}
}
