package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/get-phanuwat-api1/backend-challenge/config"
	"github.com/get-phanuwat-api1/backend-challenge/internal/container"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/scheduler"
	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/security"
	"github.com/get-phanuwat-api1/backend-challenge/migrations"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/database"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
)

// @title						User Management API
// @version					1.0
// @description				User Management API built with hexagonal architecture (ports & adapters): Gin, MongoDB and JWT (HS256).
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and the JWT from /api/v1/auth/login.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log, err := logger.NewLogger(os.Stdout, logger.Options{
		Level:   cfg.Log.Level,
		Format:  cfg.Log.Format,
		Service: cfg.App.Name,
	})
	if err != nil {
		return err
	}
	slog.SetDefault(log)
	if cfg.App.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}

	// ctx is cancelled on SIGINT/SIGTERM, which starts the graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Database -----------------------------------------------------------
	connectCtx, cancelConnect := context.WithTimeout(ctx, cfg.Database.ConnectTimeout)
	defer cancelConnect()
	client, err := database.NewMongoClient(connectCtx, cfg.Database.URI)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Error("disconnect mongo", "error", err)
		}
	}()
	db := client.Database(cfg.Database.Name)

	// Every deployment starts the API, so pending migrations are applied on each
	// deploy. A lock in MongoDB keeps replicas from migrating at the same time.
	if cfg.Database.AutoMigrate {
		log.Info("running database migrations")
		applied, err := database.MigrateUp(ctx, db, migrations.FS)
		if err != nil {
			return fmt.Errorf("auto-migrate: %w", err)
		}
		for _, m := range applied {
			log.Info("migration applied", "version", m.Version, "description", m.Description)
		}
		log.Info("database schema is up to date", "applied", len(applied))
	}

	// --- Wiring -------------------------------------------------------------
	tokens, err := security.NewJWTManager([]byte(cfg.JWT.Secret), cfg.JWT.Issuer, cfg.JWT.TTL)
	if err != nil {
		return err
	}
	c := container.NewContainer(db, security.NewBcryptHasher(bcrypt.DefaultCost), tokens, log)

	// --- Servers ------------------------------------------------------------
	httpPort := strconv.Itoa(cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:              ":" + httpPort,
		Handler:           c.Router,
		ReadHeaderTimeout: cfg.Server.ReadTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
	}
	grpcLis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.GRPCPort))
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	serverErr := make(chan error, 2)
	go func() {
		log.Info("http server started", "addr", srv.Addr, "swagger", "http://localhost:"+httpPort+"/api-docs")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("http server: %w", err)
		}
	}()
	go func() {
		log.Info("grpc server started", "addr", grpcLis.Addr().String())
		if err := c.GRPCServer.Serve(grpcLis); err != nil {
			serverErr <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	// --- Background jobs ----------------------------------------------------
	var wg sync.WaitGroup
	wg.Go(func() {
		scheduler.RunUserCountLogger(ctx, c.UserService, cfg.Worker.UserCountInterval, log.With("layer", "scheduler"))
	})

	// --- Graceful shutdown --------------------------------------------------
	var runErr error
	select {
	case runErr = <-serverErr:
	case <-ctx.Done():
	}
	stop() // also stops the background worker when a server failed

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http graceful shutdown", "error", err)
	}
	grpcStopped := make(chan struct{})
	go func() {
		c.GRPCServer.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		c.GRPCServer.Stop()
	}
	wg.Wait()

	log.Info("server stopped")
	return runErr
}
