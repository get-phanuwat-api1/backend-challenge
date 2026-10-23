package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/httpapi"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

const testUserID = "6ad83e6db6f99c0a372a5356"

var testUser = &domain.User{
	ID:           testUserID,
	Name:         "Alice",
	Email:        "alice@example.com",
	PasswordHash: "$2a$10$secret-hash",
	CreatedAt:    time.Date(2026, 10, 21, 4, 24, 13, 532_000_000, time.UTC),
}

// newEngine registers the handlers without the auth middleware; the
// authenticated user is injected directly into the context instead.
func newEngine(authSvc *mocks.AuthService, userSvc *mocks.UserService) *gin.Engine {
	log := logger.NewDiscardLogger()
	ah := httpapi.NewAuthHandler(authSvc, log)
	uh := httpapi.NewUserHandler(userSvc, log)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(requestctx.WithUserID(c.Request.Context(), testUserID))
	})
	r.POST("/auth/register", ah.Register)
	r.POST("/auth/login", ah.Login)
	r.POST("/users", uh.CreateUser)
	r.GET("/users", uh.ListUsers)
	r.GET("/users/me", uh.GetMe)
	r.GET("/users/:id", uh.GetUserByID)
	r.PATCH("/users/:id", uh.PatchUserByID)
	r.DELETE("/users/:id", uh.DeleteUserByID)
	return r
}

func do(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, rd))
	return w
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if w.Code != status {
		t.Errorf("status = %d, want %d", w.Code, status)
	}
	var got, want any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", w.Body.String(), err)
	}
	if err := json.Unmarshal([]byte(body), &want); err != nil {
		t.Fatalf("invalid expected JSON: %v", err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("body mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

const userJSON = `{"id":"6ad83e6db6f99c0a372a5356","name":"Alice","email":"alice@example.com","created_at":"2026-10-21T04:24:13.532Z"}`

func errorJSON(code string) string {
	return `{"successful":false,"error_code":"` + code + `","data":null}`
}
