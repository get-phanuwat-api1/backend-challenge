package grpcapi_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/grpcapi"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
	userv1 "github.com/get-phanuwat-api1/backend-challenge/proto/user/v1"
)

// newTestClient runs the real gRPC adapter over an in-memory listener, backed
// by the real application services and fakes for the driven ports.
func newTestClient(t *testing.T) (userv1.UserServiceClient, string) {
	t.Helper()
	log := logger.NewDiscardLogger()
	repo, hasher, tokens := mocks.NewUserRepositoryFake(), mocks.PasswordHasherFake{}, mocks.TokenManagerFake{}
	userSvc := application.NewUserService(repo, hasher, log)
	authSvc := application.NewAuthService(userSvc, repo, hasher, tokens, log)
	srv := grpcapi.NewServer(userSvc, authSvc, log)

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	tok, err := tokens.Generate("caller")
	if err != nil {
		t.Fatal(err)
	}
	return userv1.NewUserServiceClient(conn), tok.Value
}

func authCtx(tok string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tok)
}

func assertStatus(t *testing.T, err error, wantCode codes.Code, wantMsg string) {
	t.Helper()
	st := status.Convert(err)
	if st.Code() != wantCode || (wantMsg != "" && st.Message() != wantMsg) {
		t.Fatalf("status = %v %q, want %v %q", st.Code(), st.Message(), wantCode, wantMsg)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	client, tok := newTestClient(t)
	ctx := authCtx(tok)

	created, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "Alice", Email: "Alice@Example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.GetUser().GetId() == "" || created.GetUser().GetCreatedAt() == nil || created.GetUser().GetEmail() != "alice@example.com" {
		t.Fatalf("unexpected user: %v", created.GetUser())
	}

	got, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: created.GetUser().GetId()})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.GetUser().GetName() != "Alice" {
		t.Fatalf("got %v", got.GetUser())
	}
}

func TestErrorCodes(t *testing.T) {
	client, tok := newTestClient(t)
	ctx := authCtx(tok)
	if _, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "A", Email: "a@example.com", Password: "password123"}); err != nil {
		t.Fatal(err)
	}

	_, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "A", Email: "bad", Password: "password123"})
	assertStatus(t, err, codes.InvalidArgument, "INVALID_EMAIL")

	_, err = client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "A", Email: "a@example.com", Password: "password123"})
	assertStatus(t, err, codes.AlreadyExists, "EMAIL_ALREADY_EXISTS")

	_, err = client.GetUser(ctx, &userv1.GetUserRequest{Id: "missing"})
	assertStatus(t, err, codes.NotFound, "USER_NOT_FOUND")
}

func TestAuthInterceptor(t *testing.T) {
	client, _ := newTestClient(t)

	_, err := client.GetUser(context.Background(), &userv1.GetUserRequest{Id: "x"})
	assertStatus(t, err, codes.Unauthenticated, "UNAUTHORIZED")

	_, err = client.GetUser(authCtx("invalid"), &userv1.GetUserRequest{Id: "x"})
	assertStatus(t, err, codes.Unauthenticated, "UNAUTHORIZED")

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic abc")
	_, err = client.GetUser(ctx, &userv1.GetUserRequest{Id: "x"})
	assertStatus(t, err, codes.Unauthenticated, "UNAUTHORIZED")
}
