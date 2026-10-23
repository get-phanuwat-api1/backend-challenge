// Package mocks contains test doubles for the ports: fakes for the driven
// ports (repository, hasher, token manager) and mocks for the driving ports.
package mocks

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

// UserRepositoryFake is a concurrency-safe in-memory application.UserRepository that
// mimics the MongoDB adapter, including the unique email index.
type UserRepositoryFake struct {
	mu     sync.Mutex
	users  map[string]domain.User
	nextID int

	// Err, when set, is returned by every method to simulate a database failure.
	Err error
	// CountCalls records how many times CountUsers was called.
	CountCalls int
}

var _ application.UserRepository = (*UserRepositoryFake)(nil)

// NewUserRepositoryFake returns an empty fake repository.
func NewUserRepositoryFake() *UserRepositoryFake {
	return &UserRepositoryFake{users: make(map[string]domain.User)}
}

func (r *UserRepositoryFake) emailTaken(email, exceptID string) bool {
	for id, u := range r.users {
		if u.Email == email && id != exceptID {
			return true
		}
	}
	return false
}

func (r *UserRepositoryFake) CreateUser(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	if r.emailTaken(u.Email, "") {
		return domain.ErrEmailAlreadyExists
	}
	r.nextID++
	u.ID = fmt.Sprintf("%024x", r.nextID) // looks like a MongoDB ObjectID
	r.users[u.ID] = *u
	return nil
}

func (r *UserRepositoryFake) GetUserByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return &u, nil
}

func (r *UserRepositoryFake) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	for _, u := range r.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *UserRepositoryFake) ListUsers(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	users := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}
	slices.SortFunc(users, func(a, b domain.User) int { return cmp.Compare(a.ID, b.ID) })
	return users, nil
}

func (r *UserRepositoryFake) UpdateUser(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	existing, ok := r.users[u.ID]
	if !ok {
		return domain.ErrUserNotFound
	}
	if r.emailTaken(u.Email, u.ID) {
		return domain.ErrEmailAlreadyExists
	}
	existing.Name, existing.Email = u.Name, u.Email
	r.users[u.ID] = existing
	return nil
}

func (r *UserRepositoryFake) DeleteUserByID(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *UserRepositoryFake) CountUsers(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.CountCalls++
	if r.Err != nil {
		return 0, r.Err
	}
	return int64(len(r.users)), nil
}

// Calls returns how many times CountUsers was called (safe for concurrent use).
func (r *UserRepositoryFake) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.CountCalls
}
