package scheduler_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/scheduler"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
)

// syncBuffer is a goroutine-safe bytes.Buffer for capturing logs.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestRunUserCountLogger(t *testing.T) {
	repo := mocks.NewUserRepositoryFake()
	_ = repo.CreateUser(context.Background(), &domain.User{Email: "a@example.com"})
	_ = repo.CreateUser(context.Background(), &domain.User{Email: "b@example.com"})

	var buf syncBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.RunUserCountLogger(ctx, repo, 10*time.Millisecond, log)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(buf.String(), "total_users=2") {
		if time.Now().After(deadline) {
			t.Fatalf("user count was not logged; logs:\n%s", buf.String())
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestRunUserCountLogger_LogsErrorsAndKeepsRunning(t *testing.T) {
	repo := mocks.NewUserRepositoryFake()
	repo.Err = context.DeadlineExceeded

	var buf syncBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	scheduler.RunUserCountLogger(ctx, repo, 10*time.Millisecond, log)

	if !strings.Contains(buf.String(), "count users") {
		t.Fatalf("expected error to be logged; logs:\n%s", buf.String())
	}
	if repo.Calls() < 2 {
		t.Fatalf("worker should keep ticking after errors; calls = %d", repo.Calls())
	}
}
