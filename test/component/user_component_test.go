// Package component runs end-to-end tests within the service: real HTTP router,
// services and repository against a real MongoDB (testcontainers or TEST_MONGO_URI).
// They are skipped with `go test -short`.
package component_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"

	"github.com/get-phanuwat-api1/backend-challenge/internal/container"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/security"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
	"github.com/get-phanuwat-api1/backend-challenge/test/testutil"
)

var testDB *mongo.Database

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	flag.Parse() // testing.Short needs parsed flags
	if testing.Short() {
		fmt.Println("skipping component tests in -short mode")
		return 0
	}
	gin.SetMode(gin.TestMode)
	db, cleanup, err := testutil.StartMongo(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "start mongo:", err)
		return 1
	}
	defer cleanup()
	testDB = db
	return m.Run()
}

type envelope struct {
	Successful bool            `json:"successful"`
	ErrorCode  string          `json:"error_code"`
	Data       json.RawMessage `json:"data"`
}

type user struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type api struct {
	t   *testing.T
	srv *httptest.Server
}

func newAPI(t *testing.T) *api {
	t.Helper()
	testutil.ClearCollections(t, testDB)
	tokens, err := security.NewJWTManager([]byte("component-secret-component-secret"), "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c := container.NewContainer(testDB, security.NewBcryptHasher(bcrypt.MinCost), tokens, logger.NewDiscardLogger())
	srv := httptest.NewServer(c.Router)
	t.Cleanup(srv.Close)
	return &api{t: t, srv: srv}
}

// call sends a request and decodes the response envelope.
func (a *api) call(method, path, tok, body string, wantStatus int) envelope {
	a.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, a.srv.URL+path, rd)
	if err != nil {
		a.t.Fatal(err)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		a.t.Fatalf("%s %s: status = %d, want %d; body = %s", method, path, resp.StatusCode, wantStatus, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		a.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
	}
	return env
}

func decodeData[T any](t *testing.T, env envelope) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(env.Data, &v); err != nil {
		t.Fatalf("decode data %s: %v", env.Data, err)
	}
	return v
}

func (a *api) registerAndLogin(name, email string) (user, string) {
	a.t.Helper()
	env := a.call(http.MethodPost, "/api/v1/auth/register", "",
		`{"name":"`+name+`","email":"`+email+`","password":"password123"}`, http.StatusCreated)
	u := decodeData[user](a.t, env)

	env = a.call(http.MethodPost, "/api/v1/auth/login", "",
		`{"email":"`+email+`","password":"password123"}`, http.StatusOK)
	tok := decodeData[struct {
		AccessToken string `json:"access_token"`
	}](a.t, env)
	return u, tok.AccessToken
}

func TestUserLifecycle(t *testing.T) {
	a := newAPI(t)
	alice, tok := a.registerAndLogin("Alice", "Alice@Example.com")
	if alice.Email != "alice@example.com" || alice.CreatedAt.IsZero() {
		t.Fatalf("registered user = %+v", alice)
	}

	// Create another user through the protected endpoint.
	env := a.call(http.MethodPost, "/api/v1/users", tok, `{"name":"Bob","email":"bob@example.com","password":"password123"}`, http.StatusCreated)
	bob := decodeData[user](t, env)

	// List
	env = a.call(http.MethodGet, "/api/v1/users", tok, "", http.StatusOK)
	list := decodeData[struct {
		Users []user `json:"users"`
		Total int    `json:"total"`
	}](t, env)
	if list.Total != 2 || list.Users[0].ID != alice.ID || list.Users[1].ID != bob.ID {
		t.Fatalf("list = %+v", list)
	}

	// Get by ID and /me
	if got := decodeData[user](t, a.call(http.MethodGet, "/api/v1/users/"+bob.ID, tok, "", http.StatusOK)); got.Name != "Bob" {
		t.Fatalf("get = %+v", got)
	}
	if got := decodeData[user](t, a.call(http.MethodGet, "/api/v1/users/me", tok, "", http.StatusOK)); got.ID != alice.ID {
		t.Fatalf("me = %+v", got)
	}

	// Patch own account; changes are persisted
	a.call(http.MethodPatch, "/api/v1/users/"+alice.ID, tok, `{"name":"Alicia"}`, http.StatusOK)
	if got := decodeData[user](t, a.call(http.MethodGet, "/api/v1/users/"+alice.ID, tok, "", http.StatusOK)); got.Name != "Alicia" {
		t.Fatalf("patch not persisted: %+v", got)
	}

	// Delete own account
	if env := a.call(http.MethodDelete, "/api/v1/users/"+alice.ID, tok, "", http.StatusOK); !env.Successful {
		t.Fatalf("delete = %+v", env)
	}
	a.call(http.MethodGet, "/api/v1/users/"+alice.ID, tok, "", http.StatusNotFound)
}

func TestErrorCodes(t *testing.T) {
	a := newAPI(t)
	alice, tok := a.registerAndLogin("Alice", "alice@example.com")
	bob, _ := a.registerAndLogin("Bob", "bob@example.com")

	tests := []struct {
		name, method, path, tok, body string
		status                        int
		code                          string
	}{
		{"duplicate email (case-insensitive)", http.MethodPost, "/api/v1/auth/register", "", `{"name":"A","email":"ALICE@example.com","password":"password123"}`, http.StatusConflict, "EMAIL_ALREADY_EXISTS"},
		{"invalid email", http.MethodPost, "/api/v1/auth/register", "", `{"name":"A","email":"nope","password":"password123"}`, http.StatusBadRequest, "INVALID_EMAIL"},
		{"wrong password", http.MethodPost, "/api/v1/auth/login", "", `{"email":"alice@example.com","password":"wrong-pass"}`, http.StatusUnauthorized, "INVALID_CREDENTIALS"},
		{"no token", http.MethodGet, "/api/v1/users", "", "", http.StatusUnauthorized, "UNAUTHORIZED"},
		{"bad token", http.MethodGet, "/api/v1/users", "garbage", "", http.StatusUnauthorized, "UNAUTHORIZED"},
		{"malformed id", http.MethodGet, "/api/v1/users/xyz", tok, "", http.StatusBadRequest, "INVALID_USER_ID"},
		{"unknown id", http.MethodGet, "/api/v1/users/000000000000000000000000", tok, "", http.StatusNotFound, "USER_NOT_FOUND"},
		{"patch someone else", http.MethodPatch, "/api/v1/users/" + bob.ID, tok, `{"name":"Hacked"}`, http.StatusForbidden, "FORBIDDEN"},
		{"delete someone else", http.MethodDelete, "/api/v1/users/" + bob.ID, tok, "", http.StatusForbidden, "FORBIDDEN"},
		{"patch to taken email", http.MethodPatch, "/api/v1/users/" + alice.ID, tok, `{"email":"bob@example.com"}`, http.StatusConflict, "EMAIL_ALREADY_EXISTS"},
		{"empty patch", http.MethodPatch, "/api/v1/users/" + alice.ID, tok, `{}`, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := a.call(tt.method, tt.path, tt.tok, tt.body, tt.status)
			if env.Successful || env.ErrorCode != tt.code || string(env.Data) != "null" {
				t.Fatalf("envelope = %+v, want error_code %s", env, tt.code)
			}
		})
	}
}
