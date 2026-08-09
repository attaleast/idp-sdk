package logging

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func GinMiddleware(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		logger := WithTrace(c.Request.Context(), base)
		if reqID := c.GetHeader("X-Request-Id"); reqID != "" {
			logger = logger.With(slog.String("request_id", reqID))
		}
		c.Request = c.Request.WithContext(WithContext(c.Request.Context(), logger))

		c.Next()

		fields := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, slog.String("gin_errors", c.Errors.String()))
		}

		switch {
		case c.Writer.Status() >= 500:
			logger.Error("request", fields...)
		case c.Writer.Status() >= 400:
			logger.Warn("request", fields...)
		default:
			logger.Info("request", fields...)
		}
	}
}
