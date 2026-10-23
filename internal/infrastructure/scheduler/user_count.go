// Package scheduler is a driving adapter that triggers use cases on a timer
// instead of on an HTTP or gRPC request.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
)

// RunUserCountLogger logs the total number of users every interval until ctx
// is cancelled. It blocks, so start it in its own goroutine.
func RunUserCountLogger(ctx context.Context, counter application.UserCounter, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("user count logger stopped")
			return
		case <-ticker.C:
			countCtx, cancel := context.WithTimeout(ctx, interval)
			n, err := counter.CountUsers(countCtx)
			cancel()
			if err != nil {
				logger.Error("count users", slog.Any("error", err))
				continue
			}
			logger.Info("user count", slog.Int64("total_users", n))
		}
	}
}
