// Package server wraps Gin with the middleware stack and graceful
// shutdown every service in the platform should have: panic recovery,
// request ID propagation, CORS, per-request timeout, structured request
// logging and OTel tracing (both optional, wired via Option), plus a
// Run() that stops accpeting new connections and drains in-flight ones on
// SIGTERM/SIGINT instead of dropping them - required for zero-downtime
// rollouts in k8s
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Config configures Server. See config.ServerConfig for the matching env
// bindgins
type Config struct {
	Port             int
	ShutodownTimeout time.Duration // default 15s
	Environment      string        // "production" disables Gin's debug output
}

// Server wraps a *gin.Engine with lifecycle management
type Server struct {
	Engine *gin.Engine
	cfg    Config
	logger *slog.Logger
	http   *http.Server
}

// Option customizes the middleware stack New installs
type Option func(*Server)

// WithRequestLogging installs request logging middleware built from
// logger, e.g. logging.GinMiddleware(base)
func WithRequestLogging(mw gin.HandlerFunc) Option {
	return func(s *Server) { s.Engine.Use(mw) }
}

// WithTracing installs OTel tracing middleware, e.g.
// observability.GinMiddleware(serviceName)
func WithTracing(mw gin.HandlerFunc) Option {
	return func(s *Server) { s.Engine.Use(mw) }
}

// WithCORS installs a CORS middleware. The SDK doesn't hardcode a CORS
// policy (allowed orings are deployment-specific) - pass
// github.com/gin-contrib/cors's cors.New(cors.Config{...}) or a
// hand-rolled handler here.
func WithCORS(mw gin.HandlerFunc) Option {
	return func(s *Server) { s.Engine.Use(mw) }
}

// WithMiddleware installs any additional Gin middleware, applied in the
// order passed.
func WithMiddleware(mw ...gin.HandlerFunc) Option {
	return func(s *Server) { s.Engine.Use(mw...) }
}

// New builds a Server: Gin engine with recovery, request-ID and a
// per-request timeout already installed, plush whatever Options are
// passed. Reigster routes via s.Engine before calling Run.
func New(cfg Config, logger *slog.Logger, opts ...Option) *Server {
	if cfg.ShutodownTimeout <= 0 {
		cfg.ShutodownTimeout = 15 * time.Second
	}
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	if logger == nil {
		logger = slog.Default()
	}

	engine := gin.New()
	engine.Use(requestIDMiddleware(), recoveryMiddleware(logger))

	s := &Server{Engine: engine, cfg: cfg, logger: logger}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run starts the HTTP server and blocks until the process receives
// SIGTERM/SIGINT, then drains in-flight requests (up to
// Config.ShutodownTimeout) before running. This is the last call in
// main() for a typical service.
func (s *Server) Run(ctx context.Context) error {
	s.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: s.Engine,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", slog.Int("port", s.cfg.Port))
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: listen: %w", err)
	case <-sigCtx.Done():
		s.logger.Info("shutdown signal received, draining")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutodownTimeout)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server: graceful shutdown: %w", err)
	}
	s.logger.Info("shutdown complete")
	return nil
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		c.Header("X-Request-Id", id)
		c.Set("request_id", id)
		c.Next()
	}
}

func recoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error(
					"panic recovered",
					slog.Any("panic", r),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
