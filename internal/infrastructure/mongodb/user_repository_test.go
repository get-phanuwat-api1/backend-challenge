package mongodb_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/mongodb"
	"github.com/get-phanuwat-api1/backend-challenge/test/testutil"
)

// Repository tests run against a real MongoDB (testcontainers, or TEST_MONGO_URI).
// They are skipped with `go test -short`.
var testDB *mongo.Database

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	flag.Parse() // testing.Short needs parsed flags
	if testing.Short() {
		fmt.Println("skipping repository tests in -short mode")
		return 0
	}
	db, cleanup, err := testutil.StartMongo(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "start mongo:", err)
		return 1
	}
	defer cleanup()
	testDB = db
	return m.Run()
}

func newRepo(t *testing.T) application.UserRepository {
	t.Helper()
	testutil.ClearCollections(t, testDB)
	return mongodb.NewUserRepository(testDB)
}

func newUser(name, email string) *domain.User {
	return &domain.User{Name: name, Email: email, PasswordHash: "hash", CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
}

func assertCode(t *testing.T, err error, want domain.ErrorCode) {
	t.Helper()
	if got := domain.CodeOf(err); err == nil || got != want {
		t.Fatalf("error = %v (code %s), want code %s", err, got, want)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	alice := newUser("Alice", "alice@example.com")
	if err := repo.CreateUser(ctx, alice); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if len(alice.ID) != 24 {
		t.Fatalf("CreateUser should set a hex ObjectID, got %q", alice.ID)
	}

	got, err := repo.GetUserByID(ctx, alice.ID)
	if err != nil || got.Email != alice.Email || got.PasswordHash != "hash" || !got.CreatedAt.Equal(alice.CreatedAt) {
		t.Fatalf("GetUserByID = %+v, %v", got, err)
	}
	if got, err := repo.GetUserByEmail(ctx, "alice@example.com"); err != nil || got.ID != alice.ID {
		t.Fatalf("GetUserByEmail = %+v, %v", got, err)
	}

	_, err = repo.GetUserByID(ctx, "not-an-object-id")
	assertCode(t, err, domain.CodeInvalidUserID)
	_, err = repo.GetUserByID(ctx, "000000000000000000000000")
	assertCode(t, err, domain.CodeUserNotFound)
	_, err = repo.GetUserByEmail(ctx, "nobody@example.com")
	assertCode(t, err, domain.CodeUserNotFound)
}

func TestCreateUser_UniqueEmail(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if err := repo.CreateUser(ctx, newUser("Alice", "alice@example.com")); err != nil {
		t.Fatal(err)
	}
	assertCode(t, repo.CreateUser(ctx, newUser("Other", "alice@example.com")), domain.CodeEmailAlreadyExists)
}

func TestListAndCountUsers(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	if users, err := repo.ListUsers(ctx); err != nil || len(users) != 0 {
		t.Fatalf("empty ListUsers = %v, %v", users, err)
	}
	alice, bob := newUser("Alice", "alice@example.com"), newUser("Bob", "bob@example.com")
	for _, u := range []*domain.User{alice, bob} {
		if err := repo.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	users, err := repo.ListUsers(ctx)
	if err != nil || len(users) != 2 || users[0].ID != alice.ID || users[1].ID != bob.ID {
		t.Fatalf("ListUsers = %+v, %v", users, err)
	}
	if n, err := repo.CountUsers(ctx); err != nil || n != 2 {
		t.Fatalf("CountUsers = %d, %v", n, err)
	}
}

func TestUpdateUser(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	alice, bob := newUser("Alice", "alice@example.com"), newUser("Bob", "bob@example.com")
	for _, u := range []*domain.User{alice, bob} {
		if err := repo.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	alice.Name, alice.Email = "Alicia", "alicia@example.com"
	if err := repo.UpdateUser(ctx, alice); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, _ := repo.GetUserByID(ctx, alice.ID)
	if got.Name != "Alicia" || got.Email != "alicia@example.com" || got.PasswordHash != "hash" {
		t.Fatalf("update not persisted correctly: %+v", got)
	}

	alice.Email = "bob@example.com"
	assertCode(t, repo.UpdateUser(ctx, alice), domain.CodeEmailAlreadyExists)

	ghost := newUser("Ghost", "ghost@example.com")
	ghost.ID = "000000000000000000000000"
	assertCode(t, repo.UpdateUser(ctx, ghost), domain.CodeUserNotFound)
	ghost.ID = "bad"
	assertCode(t, repo.UpdateUser(ctx, ghost), domain.CodeInvalidUserID)
}

func TestDeleteUserByID(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	alice := newUser("Alice", "alice@example.com")
	if err := repo.CreateUser(ctx, alice); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteUserByID(ctx, alice.ID); err != nil {
		t.Fatalf("DeleteUserByID: %v", err)
	}
	assertCode(t, repo.DeleteUserByID(ctx, alice.ID), domain.CodeUserNotFound)
	assertCode(t, repo.DeleteUserByID(ctx, "bad"), domain.CodeInvalidUserID)
}
