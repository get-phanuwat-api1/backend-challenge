package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/backend-challenge/internal/infrastructure/httpapi"
	"github.com/get-phanuwat-api1/backend-challenge/internal/mocks"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/logger"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

// fakeAuth accepts only the token "good" and maps it to "user-1".
var fakeAuth = &mocks.AuthService{
	VerifyAccessTokenFunc: func(tok string) (string, error) {
		if tok == "good" {
			return "user-1", nil
		}
		return "", errors.New("invalid")
	},
}

func newMiddlewareEngine(t *testing.T, buf *bytes.Buffer) *gin.Engine {
	t.Helper()
	l, err := logger.NewLogger(buf, logger.Options{Level: "debug"})
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(httpapi.RequestID(), httpapi.AccessLog(l, "/health"), httpapi.Recovery(l))
	r.GET("/echo", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"request_id": requestctx.RequestID(c.Request.Context())})
	})
	r.GET("/private", httpapi.Auth(fakeAuth), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": requestctx.UserID(c.Request.Context())})
	})
	r.GET("/bad", func(c *gin.Context) {
		c.Set(httpapi.ContextKeyErrorCode, "INVALID_NAME")
		c.Status(http.StatusBadRequest)
	})
	r.GET("/panic", func(c *gin.Context) { panic("boom") })
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func get(r http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestRequestID_PropagatesIncomingHeader(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := get(newMiddlewareEngine(t, &buf), "/echo", map[string]string{"X-Request-ID": "abc-123"})

	if got := w.Header().Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("X-Request-ID = %q", got)
	}
	if !strings.Contains(w.Body.String(), `"request_id":"abc-123"`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestRequestID_GeneratesWhenMissingOrUnsafe(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := newMiddlewareEngine(t, &buf)

	for _, incoming := range []string{"", "bad value\nwith newline"} {
		id := get(r, "/echo", map[string]string{"X-Request-ID": incoming}).Header().Get("X-Request-ID")
		if len(id) != 32 || id == incoming {
			t.Errorf("incoming %q: generated id = %q", incoming, id)
		}
	}
}

func TestAuth(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := newMiddlewareEngine(t, &buf)

	w := get(r, "/private", map[string]string{"Authorization": "Bearer good"})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"user_id":"user-1"`) {
		t.Fatalf("valid token: %d %s", w.Code, w.Body.String())
	}

	for _, header := range []string{"", "Bearer bad", "Basic good", "good"} {
		w := get(r, "/private", map[string]string{"Authorization": header})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status = %d, want 401", header, w.Code)
		}
		if w.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("Authorization %q: missing WWW-Authenticate", header)
		}
		const want = `{"successful":false,"error_code":"UNAUTHORIZED","data":null}`
		if strings.TrimSpace(w.Body.String()) != want {
			t.Errorf("Authorization %q: body = %s", header, w.Body.String())
		}
	}
}

func TestAccessLog_WritesRequestLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	get(newMiddlewareEngine(t, &buf), "/private", map[string]string{"X-Request-ID": "req-9", "Authorization": "Bearer good"})

	lines := logLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1", len(lines))
	}
	l := lines[0]
	for key, want := range map[string]any{
		"msg": "http request", "level": "INFO", "method": "GET", "route": "/private",
		"path": "/private", "status": float64(200), "request_id": "req-9", "user_id": "user-1",
	} {
		if l[key] != want {
			t.Errorf("%s = %v, want %v", key, l[key], want)
		}
	}
	if _, ok := l["latency_ms"]; !ok {
		t.Error("missing latency_ms")
	}
}

func TestAccessLog_WarnOn4xxWithErrorCode(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	get(newMiddlewareEngine(t, &buf), "/bad", nil)

	lines := logLines(t, &buf)
	if len(lines) != 1 || lines[0]["level"] != "WARN" || lines[0]["error_code"] != "INVALID_NAME" {
		t.Fatalf("unexpected log: %v", lines)
	}
}

func TestAccessLog_SkipsPrefixes(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	get(newMiddlewareEngine(t, &buf), "/health", nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no log, got %s", buf.String())
	}
}

func TestRecovery_Returns500AndLogsPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := get(newMiddlewareEngine(t, &buf), "/panic", map[string]string{"X-Request-ID": "req-p"})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	if want := `{"successful":false,"error_code":"INTERNAL_SERVER_ERROR","data":null}`; strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("body = %s", w.Body.String())
	}

	lines := logLines(t, &buf)
	if len(lines) != 2 { // panic line + access log line
		t.Fatalf("got %d log lines, want 2", len(lines))
	}
	if lines[0]["msg"] != "panic recovered" || lines[0]["panic"] != "boom" || lines[0]["request_id"] != "req-p" {
		t.Errorf("panic log = %v", lines[0])
	}
	if lines[1]["level"] != "ERROR" || lines[1]["status"] != float64(500) {
		t.Errorf("access log = %v", lines[1])
	}
}
