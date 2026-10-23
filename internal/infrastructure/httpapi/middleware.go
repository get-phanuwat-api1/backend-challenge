// Gin middleware: request ID, access log (method, path, latency), panic
// recovery and bearer-token authentication.

package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/bearer"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

// HeaderRequestID carries the correlation ID of a request.
const HeaderRequestID = "X-Request-ID"

// ContextKeyErrorCode is set by handlers so the access log can include the business error code.
const ContextKeyErrorCode = "error_code"

// safeHeaderValue limits client-supplied IDs to a safe charset and length (prevents log injection).
var safeHeaderValue = regexp.MustCompile(`^[A-Za-z0-9._:@-]{1,128}$`)

// RequestID propagates X-Request-ID (or generates one), stores it in the request
// context and echoes it in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !safeHeaderValue.MatchString(id) {
			id = newRequestID()
		}
		c.Request = c.Request.WithContext(requestctx.WithRequestID(c.Request.Context(), id))
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AccessLog writes one structured log line per HTTP request with the method,
// path and execution time. 5xx is logged at ERROR, 4xx at WARN, everything
// else at INFO. Paths starting with any of skipPrefixes (e.g. /health) are not logged.
func AccessLog(logger *slog.Logger, skipPrefixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, p := range skipPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = path // unmatched route (404)
		}

		level := slog.LevelInfo
		switch {
		case status >= http.StatusInternalServerError:
			level = slog.LevelError
		case status >= http.StatusBadRequest:
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Float64("latency_ms", float64(latency.Microseconds())/1000),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("response_bytes", c.Writer.Size()),
		}
		if code, ok := c.Get(ContextKeyErrorCode); ok {
			attrs = append(attrs, slog.Any("error_code", code))
		}
		logger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// Recovery turns panics into a logged error and a 500 response in the API's standard shape.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(c.Request.Context(), "panic recovered",
					slog.String("panic", fmt.Sprint(r)),
					slog.String("stack", string(debug.Stack())),
				)
				c.Set(ContextKeyErrorCode, string(domain.CodeInternal))
				c.AbortWithStatusJSON(http.StatusInternalServerError, NewErrorResponse(string(domain.CodeInternal)))
			}
		}()
		c.Next()
	}
}

// Auth requires a valid "Authorization: Bearer <jwt>" header. Token checks
// are delegated to the AuthService use case, so this adapter never touches
// JWT itself. The authenticated user ID is stored in the request context
// (see requestctx.UserID).
func Auth(authService application.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearer.Parse(c.GetHeader("Authorization"))
		if !ok {
			abortUnauthorized(c)
			return
		}
		userID, err := authService.VerifyAccessToken(raw)
		if err != nil {
			abortUnauthorized(c)
			return
		}
		c.Request = c.Request.WithContext(requestctx.WithUserID(c.Request.Context(), userID))
		c.Next()
	}
}

func abortUnauthorized(c *gin.Context) {
	c.Set(ContextKeyErrorCode, string(domain.CodeUnauthorized))
	c.Header("WWW-Authenticate", `Bearer realm="api"`)
	c.AbortWithStatusJSON(http.StatusUnauthorized, NewErrorResponse(string(domain.CodeUnauthorized)))
}
