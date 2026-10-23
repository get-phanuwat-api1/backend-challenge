package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
)

var errBoom = errors.New("connection refused: mongo-secret-host:27017")

func TestCreateUser(t *testing.T) {
	userSvc := &mocks.UserService{
		CreateUserFunc: func(_ context.Context, input application.CreateUserInput) (*domain.User, error) {
			if input.Email == "taken@example.com" {
				return nil, domain.ErrEmailAlreadyExists
			}
			return testUser, nil
		},
	}
	r := newEngine(nil, userSvc)

	w := do(r, http.MethodPost, "/users", `{"name":"Alice","email":"alice@example.com","password":"password123"}`)
	expect(t, w, http.StatusCreated, `{"successful":true,"error_code":"","data":`+userJSON+`}`)
	if strings.Contains(w.Body.String(), "secret-hash") {
		t.Fatal("password hash leaked in response")
	}

	w = do(r, http.MethodPost, "/users", `{"name":"Alice","email":"taken@example.com","password":"password123"}`)
	expect(t, w, http.StatusConflict, errorJSON("EMAIL_ALREADY_EXISTS"))
}

func TestListUsers(t *testing.T) {
	userSvc := &mocks.UserService{
		ListUsersFunc: func(context.Context) ([]domain.User, error) { return []domain.User{*testUser}, nil },
	}
	w := do(newEngine(nil, userSvc), http.MethodGet, "/users", "")
	expect(t, w, http.StatusOK, `{"successful":true,"error_code":"","data":{"users":[`+userJSON+`],"total":1}}`)
}

func TestListUsers_Empty(t *testing.T) {
	userSvc := &mocks.UserService{
		ListUsersFunc: func(context.Context) ([]domain.User, error) { return nil, nil },
	}
	w := do(newEngine(nil, userSvc), http.MethodGet, "/users", "")
	expect(t, w, http.StatusOK, `{"successful":true,"error_code":"","data":{"users":[],"total":0}}`)
}

func TestListUsers_InternalErrorIsHidden(t *testing.T) {
	userSvc := &mocks.UserService{
		ListUsersFunc: func(context.Context) ([]domain.User, error) { return nil, errBoom },
	}
	w := do(newEngine(nil, userSvc), http.MethodGet, "/users", "")
	expect(t, w, http.StatusInternalServerError, errorJSON("INTERNAL_SERVER_ERROR"))
	if strings.Contains(w.Body.String(), "mongo-secret-host") {
		t.Fatal("internal error leaked to client")
	}
}

func TestGetUser(t *testing.T) {
	userSvc := &mocks.UserService{
		GetUserByIDFunc: func(_ context.Context, id string) (*domain.User, error) {
			switch id {
			case testUserID:
				return testUser, nil
			case "xyz":
				return nil, domain.ErrInvalidUserID
			default:
				return nil, domain.ErrUserNotFound
			}
		},
	}
	r := newEngine(nil, userSvc)

	expect(t, do(r, http.MethodGet, "/users/"+testUserID, ""), http.StatusOK, `{"successful":true,"error_code":"","data":`+userJSON+`}`)
	expect(t, do(r, http.MethodGet, "/users/me", ""), http.StatusOK, `{"successful":true,"error_code":"","data":`+userJSON+`}`)
	expect(t, do(r, http.MethodGet, "/users/xyz", ""), http.StatusBadRequest, errorJSON("INVALID_USER_ID"))
	expect(t, do(r, http.MethodGet, "/users/000000000000000000000000", ""), http.StatusNotFound, errorJSON("USER_NOT_FOUND"))
}

func TestPatchUserByID(t *testing.T) {
	var gotID string
	var gotPatch domain.UserPatch
	userSvc := &mocks.UserService{
		PatchUserByIDFunc: func(_ context.Context, actorID, id string, patch domain.UserPatch) (*domain.User, error) {
			if actorID != id {
				return nil, domain.ErrForbidden
			}
			gotID, gotPatch = id, patch
			u := *testUser
			if err := u.ApplyPatch(patch); err != nil {
				return nil, err
			}
			return &u, nil
		},
	}
	r := newEngine(nil, userSvc)

	w := do(r, http.MethodPatch, "/users/"+testUserID, `{"name":"Alicia"}`)
	expect(t, w, http.StatusOK, `{"successful":true,"error_code":"","data":`+strings.Replace(userJSON, `"Alice"`, `"Alicia"`, 1)+`}`)
	if gotID != testUserID || gotPatch.Name == nil || *gotPatch.Name != "Alicia" || gotPatch.Email != nil {
		t.Errorf("service called with id=%q patch=%+v", gotID, gotPatch)
	}

	expect(t, do(r, http.MethodPatch, "/users/"+testUserID, `{}`), http.StatusBadRequest, errorJSON("INVALID_REQUEST_BODY"))
	expect(t, do(r, http.MethodPatch, "/users/"+testUserID, `{"email":"bad"}`), http.StatusBadRequest, errorJSON("INVALID_EMAIL"))
	expect(t, do(r, http.MethodPatch, "/users/"+testUserID, `{"password":"x"}`), http.StatusBadRequest, errorJSON("INVALID_REQUEST_BODY"))
	expect(t, do(r, http.MethodPatch, "/users/someone-else", `{"name":"X"}`), http.StatusForbidden, errorJSON("FORBIDDEN"))
}

func TestDeleteUserByID(t *testing.T) {
	userSvc := &mocks.UserService{
		DeleteUserByIDFunc: func(_ context.Context, actorID, id string) error {
			if actorID != id {
				return domain.ErrForbidden
			}
			return nil
		},
	}
	r := newEngine(nil, userSvc)

	expect(t, do(r, http.MethodDelete, "/users/"+testUserID, ""), http.StatusOK, `{"successful":true,"error_code":""}`)
	expect(t, do(r, http.MethodDelete, "/users/someone-else", ""), http.StatusForbidden, errorJSON("FORBIDDEN"))
}
