package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/config"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestLoadFile_DefaultYAMLWithEnvOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	t.Setenv("DATABASE_URI", "mongodb://mongo:27017")
	t.Setenv("SERVER_HTTP_PORT", "9090")

	cfg, err := config.LoadFile("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.Secret != testSecret || cfg.Database.URI != "mongodb://mongo:27017" || cfg.Server.HTTPPort != 9090 {
		t.Fatalf("env overrides not applied: %+v", cfg)
	}
	if cfg.Server.GRPCPort != 50051 || cfg.JWT.TTL != 24*time.Hour || cfg.Worker.UserCountInterval != 10*time.Second || !cfg.Database.AutoMigrate {
		t.Fatalf("yaml defaults not loaded: %+v", cfg)
	}
}

func TestLoad_UsesConfigPath(t *testing.T) {
	t.Setenv("CONFIG_PATH", "config.yaml")
	t.Setenv("JWT_SECRET", testSecret)
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFile_RequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "too-short")
	_, err := config.LoadFile("config.yaml")
	if err == nil || !strings.Contains(err.Error(), "jwt.secret") {
		t.Fatalf("expected jwt.secret error, got %v", err)
	}
}

func TestLoadFile_ValidatesAllFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("server:\n  http_port: 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.LoadFile(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{"http_port", "grpc_port", "database.uri", "database.name", "jwt.secret", "jwt.ttl", "user_count_interval"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q: %v", want, err)
		}
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	if _, err := config.LoadFile("does-not-exist.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
