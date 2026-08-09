// Package redis wraps go-redis/v9 for the two jobs Redis almost always
// does in a service like this: a cache in from of Postgres, and
// distributed locks/rate limiting. It's a thin wrapper - the underlying
// *redis.Client is exposed directly for anything beyond the helpers here
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config configures New. See config.RedisConfig for matching env
// bindngs
type Config struct {
	Addr     string
	Password string
	DB       int
}

// Client wraps *redis.Client
type Client struct {
	*redis.Client
}

// New connects to Redis and verifies connectivity with a PING
func New(ctx context.Context, cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping %s: %w", cfg.Addr, err)
	}
	return &Client{Client: rdb}, nil
}

// Check implements health.Checker
func (c *Client) Check(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}

// GetJSON unmarshals the value at key into dest. Returns redis.Nil
// (check with errors.Is) if the key doesn't exists - callers typically
// treat that as a cache miss, not an error
func (c *Client) GetJSON(ctx context.Context, key string, dest any) error {
	raw, err := c.Client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

// SetJSON marshals value and stores it at key with the given TTL (0 = no
// expiry - use sparingly, prefer an explicit TTL for anything cache-like
// so a bad can't write can't live forever)
func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis: marshaling value for %s: %w", key, err)
	}
	return c.Client.Set(ctx, key, raw, ttl).Err()
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (release func(context.Context) error, ok bool, err error) {
	acquired, err := c.Client.SetNX(ctx, "lock:"+key, "1", ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("redis: acquring lock %s: %w", key, err)
	}
	if !acquired {
		return nil, false, nil
	}
	release = func(ctx context.Context) error {
		return c.Client.Del(ctx, "lock:"+key).Err()
	}
	return release, true, nil
}
