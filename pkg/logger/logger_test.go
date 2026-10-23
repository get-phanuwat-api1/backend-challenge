package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

func TestNewLogger_AddsContextAttributes(t *testing.T) {
	var buf bytes.Buffer
	l, err := logger.NewLogger(&buf, logger.Options{Level: "debug", Service: "user-service"})
	if err != nil {
		t.Fatal(err)
	}

	ctx := requestctx.WithUserID(requestctx.WithRequestID(context.Background(), "req-1"), "user-1")
	l.DebugContext(ctx, "hello")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("decode %q: %v", buf.String(), err)
	}
	for key, want := range map[string]string{"msg": "hello", "service": "user-service", "request_id": "req-1", "user_id": "user-1"} {
		if line[key] != want {
			t.Errorf("%s = %v, want %q", key, line[key], want)
		}
	}
}

func TestNewLogger_TextFormatAndLevel(t *testing.T) {
	var buf bytes.Buffer
	l, err := logger.NewLogger(&buf, logger.Options{Level: "warn", Format: "text"})
	if err != nil {
		t.Fatal(err)
	}
	l.Info("hidden")
	l.Warn("shown")
	if out := buf.String(); strings.Contains(out, "hidden") || !strings.Contains(out, "msg=shown") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestNewLogger_InvalidOptions(t *testing.T) {
	if _, err := logger.NewLogger(&bytes.Buffer{}, logger.Options{Level: "verbose"}); err == nil {
		t.Error("expected error for invalid level")
	}
	if _, err := logger.NewLogger(&bytes.Buffer{}, logger.Options{Format: "xml"}); err == nil {
		t.Error("expected error for invalid format")
	}
}
