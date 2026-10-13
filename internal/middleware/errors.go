package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WriteError maps a typed AppError onto the JSON error envelope. This is the only status mapping.
func WriteError(c *gin.Context, err error) {
	app := &AppError{}
	if !errors.As(err, &app) || app == nil || app.Code == "" {
		app = Internal("internal_error", "internal server error")
	}
	c.AbortWithStatusJSON(statusOf(app.Kind), ErrorResponse{
		Error: APIError{
			Code:    app.Code,
			Message: app.Message,
			Details: app.Details,
		},
	})
}

func statusOf(kind Kind) int {
	switch kind {
	case KindInvalid:
		return http.StatusBadRequest
	case KindUnprocessable:
		return http.StatusUnprocessableEntity
	case KindConflict:
		return http.StatusConflict
	case KindNotFound:
		return http.StatusNotFound
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
