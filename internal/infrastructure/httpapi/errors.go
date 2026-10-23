// Request decoding and error responses: domain error codes become HTTP
// statuses here, because HTTP is a delivery detail the domain must not know.

package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// decodeJSONBody requires a single JSON object body without unknown fields.
func decodeJSONBody(c *gin.Context, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("request body is required")
		}
		return domain.NewError(domain.CodeInvalidRequestBody, err.Error())
	}
	if dec.More() {
		return domain.NewError(domain.CodeInvalidRequestBody, "request body must contain a single JSON object")
	}
	return nil
}

// writeError maps err to its code, exposes it to the access log, logs the
// underlying cause of unexpected (internal) errors and writes the error envelope.
func writeError(c *gin.Context, logger *slog.Logger, err error) {
	code := domain.CodeOf(err)
	c.Set(ContextKeyErrorCode, string(code))
	if code == domain.CodeInternal {
		logger.ErrorContext(c.Request.Context(), "unexpected error", slog.String("route", c.FullPath()), slog.Any("error", err))
	} else {
		logger.DebugContext(c.Request.Context(), "request rejected", slog.String("error_code", string(code)), slog.Any("error", err))
	}
	c.JSON(httpStatus(code), NewErrorResponse(string(code)))
}

// httpStatus maps a domain error code to an HTTP status.
func httpStatus(code domain.ErrorCode) int {
	switch code {
	case domain.CodeInvalidCredentials, domain.CodeUnauthorized:
		return http.StatusUnauthorized
	case domain.CodeForbidden:
		return http.StatusForbidden
	case domain.CodeUserNotFound:
		return http.StatusNotFound
	case domain.CodeEmailAlreadyExists:
		return http.StatusConflict
	case domain.CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
