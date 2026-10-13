package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const requestIDHeader = "X-Request-ID"

// RequestID stores a request id on the context and response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Writer.Header().Set(requestIDHeader, id)
		logger := zerolog.Ctx(c.Request.Context()).With().Str("request_id", id).Logger()
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context()))
		c.Next()
	}
}

// AccessLog writes one structured line per request.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		zerolog.Ctx(c.Request.Context()).Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}

// Recovery converts a panic into the JSON error envelope.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				zerolog.Ctx(c.Request.Context()).Error().Interface("panic", rec).Msg("panic")
				WriteError(c, Internal("internal_error", "internal server error"))
			}
		}()
		c.Next()
	}
}
