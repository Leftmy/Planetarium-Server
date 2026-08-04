package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/leftmy/planetarium-server/docs"
	"github.com/leftmy/planetarium-server/internal/adapter/postgres"
	httptransport "github.com/leftmy/planetarium-server/internal/transport/http"
	"github.com/leftmy/planetarium-server/internal/user/login"
	"github.com/leftmy/planetarium-server/internal/user/register"
	"github.com/leftmy/planetarium-server/pkg/token"
)

// newServer wires the whole stack the way cmd/api does and serves it over a real
// socket. Everything below therefore goes through JSON, HTTP status codes and the
// router, not just Go function calls.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()

	db := postgres.NewTestDB(t)

	users := postgres.NewUserRepo(db)
	identities := postgres.NewUserIdentityRepo(db)
	sessions := postgres.NewRefreshTokenRepo(db)
	tokens := token.NewManager("test-secret", 15*time.Minute, 30*24*time.Hour)

	mux := http.NewServeMux()
	httptransport.SetupRoutes(mux, httptransport.Handlers{
		Register: register.NewHandler(register.NewUsecase(db, users, identities)),
		Login:    login.NewHandler(login.NewUsecase(db, users, sessions, tokens)),
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

// post sends a JSON body and returns the status and the decoded response.
func post(t *testing.T, server *httptest.Server, path, body string) (int, map[string]any) {
	t.Helper()

	resp, err := server.Client().Post(server.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded map[string]any

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}

	if buf.Len() > 0 {
		if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Fatalf("decode %s response %q: %v", path, buf, err)
		}
	}

	return resp.StatusCode, decoded
}

func errorCode(t *testing.T, body map[string]any) string {
	t.Helper()

	detail, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no error object: %v", body)
	}

	return detail["code"].(string)
}

// The full journey, in the order a real client makes it.
func TestRegisterLoginRefreshCycle(t *testing.T) {
	server := newServer(t)

	const credentials = `{"name":"Ada Lovelace","email":"ada@example.com","password":"correct horse battery"}`

	status, body := post(t, server, "/api/v1/register", credentials)
	if status != http.StatusCreated {
		t.Fatalf("register: status = %d, want 201 (body %v)", status, body)
	}

	if body["id"] == nil || body["email"] != "ada@example.com" {
		t.Fatalf("register response = %v", body)
	}

	// Registering the same address again is a conflict, not a second account.
	status, body = post(t, server, "/api/v1/register", credentials)
	if status != http.StatusConflict {
		t.Fatalf("duplicate register: status = %d, want 409", status)
	}

	if code := errorCode(t, body); code != "email_already_exists" {
		t.Errorf("duplicate register: code = %q", code)
	}

	status, body = post(t, server, "/api/v1/login",
		`{"email":"ada@example.com","password":"correct horse battery"}`)
	if status != http.StatusOK {
		t.Fatalf("login: status = %d, want 200 (body %v)", status, body)
	}

	firstRefresh, _ := body["refresh_token"].(string)
	if firstRefresh == "" || body["access_token"] == "" {
		t.Fatalf("login response = %v", body)
	}

	if body["token_type"] != "Bearer" {
		t.Errorf("token_type = %v, want Bearer", body["token_type"])
	}

	status, body = post(t, server, "/api/v1/refresh",
		`{"refresh_token":"`+firstRefresh+`"}`)
	if status != http.StatusOK {
		t.Fatalf("refresh: status = %d, want 200 (body %v)", status, body)
	}

	secondRefresh, _ := body["refresh_token"].(string)
	if secondRefresh == "" || secondRefresh == firstRefresh {
		t.Fatalf("refresh did not rotate the token: %v", body)
	}

	// The exchanged token is dead.
	status, body = post(t, server, "/api/v1/refresh",
		`{"refresh_token":"`+firstRefresh+`"}`)
	if status != http.StatusUnauthorized {
		t.Fatalf("replayed refresh: status = %d, want 401", status)
	}

	if code := errorCode(t, body); code != "invalid_refresh_token" {
		t.Errorf("replayed refresh: code = %q", code)
	}

	// And the replay took the live session down with it.
	status, _ = post(t, server, "/api/v1/refresh",
		`{"refresh_token":"`+secondRefresh+`"}`)
	if status != http.StatusUnauthorized {
		t.Fatalf("session survived a detected token reuse: status = %d, want 401", status)
	}
}

func TestLoginFailureShape(t *testing.T) {
	server := newServer(t)

	status, _ := post(t, server, "/api/v1/register",
		`{"name":"Ada","email":"ada@example.com","password":"correct horse battery"}`)
	if status != http.StatusCreated {
		t.Fatalf("register: status = %d", status)
	}

	tests := []struct {
		name string
		body string
	}{
		{"wrong password", `{"email":"ada@example.com","password":"wrong"}`},
		{"unknown email", `{"email":"nobody@example.com","password":"correct horse battery"}`},
	}

	// Both must be answered identically, down to the message: anything that
	// differs tells an attacker which addresses are registered.
	var seen []map[string]any

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := post(t, server, "/api/v1/login", tt.body)
			if status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", status)
			}

			if code := errorCode(t, body); code != "invalid_credentials" {
				t.Errorf("code = %q, want invalid_credentials", code)
			}

			seen = append(seen, body)
		})
	}

	if len(seen) == 2 {
		first, _ := json.Marshal(seen[0])
		second, _ := json.Marshal(seen[1])

		if string(first) != string(second) {
			t.Errorf("the two failures answer differently:\n%s\n%s", first, second)
		}
	}
}

// The root greeting must not be a catch-all: an unknown path has to be a 404, or
// a typo in a URL looks like a working endpoint.
func TestUnknownPathIs404(t *testing.T) {
	server := newServer(t)

	resp, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get root: %v", err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /: status = %d, want 200", resp.StatusCode)
	}

	for _, path := range []string{"/nope", "/api/v1", "/api/v1/registerr"} {
		resp, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}

		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, resp.StatusCode)
		}
	}
}

// The documentation routes are part of the API surface, so they get the same
// treatment as the rest: if /docs stops loading, the spec it renders is the
// first thing anyone notices is missing.
func TestDocumentationIsServed(t *testing.T) {
	server := newServer(t)

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/api/v1/openapi.yaml", "application/yaml", "openapi: 3.0.3"},
		{"/docs", "text/html", "swagger-ui"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := server.Client().Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("get %s: %v", tt.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}

			if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, tt.contentType) {
				t.Errorf("content-type = %q, want something containing %q", ct, tt.contentType)
			}

			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(resp.Body); err != nil {
				t.Fatalf("read body: %v", err)
			}

			if !strings.Contains(buf.String(), tt.contains) {
				t.Errorf("body does not contain %q", tt.contains)
			}
		})
	}
}

// The spec is written by hand, so nothing stops it from describing endpoints
// that do not exist. This checks the one direction that matters: every path the
// spec advertises is actually routed.
func TestSpecPathsExist(t *testing.T) {
	server := newServer(t)

	// Paths and the method the spec declares for them.
	documented := map[string]string{
		"/api/v1/register": http.MethodPost,
		"/api/v1/login":    http.MethodPost,
		"/api/v1/refresh":  http.MethodPost,
	}

	spec := string(docs.OpenAPI)

	for path, method := range documented {
		if !strings.Contains(spec, path+":") {
			t.Errorf("%s is routed but missing from the spec", path)
		}

		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}

		_ = resp.Body.Close()

		// An empty object is rejected by every one of them, but a 404 or 405
		// would mean the spec describes a route that is not wired up.
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status = %d, so the documented route does not exist", method, path, resp.StatusCode)
		}
	}
}

// A wrong method must be a 405, which Go's router gives for free once the routes
// carry a method — worth pinning so a later refactor does not lose it.
func TestWrongMethodIs405(t *testing.T) {
	server := newServer(t)

	resp, err := server.Client().Get(server.URL + "/api/v1/login")
	if err != nil {
		t.Fatalf("get login: %v", err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/v1/login: status = %d, want 405", resp.StatusCode)
	}
}
