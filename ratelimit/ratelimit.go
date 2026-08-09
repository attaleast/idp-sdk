// Package ratelimit implements a Redis-based fixed-window rate limiter:
// simple, cheap (one INCR+EXPIRE per check), and good enough for
// protecting an API endpoint (e.g. event ingestion) across multiple
// service replicas, which an in-memory limiter can't do. It trades a
// small amout of burst tolerance at window boundaries for that
// simplicity - reach for sliding-window/token-bucket algorithm instead
// if exact burst control matters than implementation simplicity
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
	prefix string
}

// New builds a Limiter. rdb is typically (*redis.Client) from cache/reids's
// Client (which embeds *redis.Client, so it satisfies this directly)
func New(rdb *redis.Client, limit int, window time.Duration, keyPrefix string) *Limiter {
	if keyPrefix == "" {
		keyPrefix = "ratelimit"
	}
	return &Limiter{rdb: rdb, limit: limit, window: window, prefix: keyPrefix}
}

// Allow reports whether the call identified by key is within the limit,
// incrementing its counter as a side effect. remaining is the number of
// calls left in current window (0 if !allowed). retryAfter is how
// long until the window resets, meaningful only when !allowed
func (l *Limiter) Allow(ctx context.Context, key string) (allowed bool, remaining int, retryAfter time.Duration, err error) {
	redisKey := fmt.Sprintf("%s:%s:%d", l.prefix, key, time.Now().Unix()/int64(l.window.Seconds()))

	count, err := l.rdb.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, 0, 0, fmt.Errorf("ratelimit: incr: %w", err)
	}
	if count == 1 {
		if err := l.rdb.Expire(ctx, redisKey, l.window).Err(); err != nil {
			return false, 0, 0, fmt.Errorf("ratelimit: expire: %w", err)
		}
	}

	ttl, err := l.rdb.TTL(ctx, redisKey).Result()
	if err != nil {
		ttl = l.window
	}

	if int(count) > l.limit {
		return false, 0, ttl, nil
	}

	return true, l.limit - int(count), ttl, nil
}

// GinMiddleware rejects requests over the limit with 429 and a
// Retry-After header. keyFunc extracts the rate-limit key from the
// request - typically the caller's API key or client IP, e.g.:
//
//	ratelimit.GinMiddleware(limiter, func (c *gin.Context) string {
//			return c.Param("project_id")
//	})
func GinMiddleware(l *Limiter, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, remaining, retryAfter, err := l.Allow(c.Request.Context(), keyFunc(c))
		if err != nil {
			// Fail open: a Redis blip shouldn't take down ingerstion.
			// Fail closed instead of here if the endpoint being protected
			// is expensive enough that "no limiter" is worse than
			// "no traffic"
			c.Next()
			return
		}
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
