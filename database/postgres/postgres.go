// Package postgres wraps pgx/v5's connection pool with the defaults a
// production service need: bounded pool size, connection lifetime
// limits (so a rolling Postgres restart or a load balancer in front of
// it doesn't leave the app stuck on dead connections), a health.Checker
// implementation, and a small transaction helper
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config configures New. See Config.PostgresConfig for the matching env
// bindings
type Config struct {
	DSN         string
	MaxConns    int32         // default 20
	MinConns    int32         // default 2
	ConnTimeout time.Duration // default 5s

	MaxConnLifetime time.Duration // default 30m
	MinConnLifetime time.Duration // default 5m
}

// DB wraps *pgxpool.Pool. Embedding the pool directly (rather than hiding
// it behind an interface) is deliberate: query code should use pgx's own
// API (pgx.CollectRows, etc.) directly rather that through a
// least-common-denominator wrapper.
type DB struct {
	*pgxpool.Pool
}

// New parsers cfg, opens a pool and verifies connectivity with a Ping
// before returning - fail fast at startup rather than on the fisrt
// request
func New(ctx context.Context, cfg Config) (*DB, error) {
	pgxCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parsing DSN: %w", err)
	}

	pgxCfg.MaxConns = orDefault(cfg.MaxConns, 20)
	pgxCfg.MinConns = orDefault(cfg.MinConns, 2)
	pgxCfg.MaxConnLifetime = orDefaultDuration(cfg.MaxConnLifetime, 30*time.Minute)
	pgxCfg.MaxConnIdleTime = orDefaultDuration(cfg.MinConnLifetime, 5*time.Minute)
	pgxCfg.HealthCheckPeriod = time.Minute
	pgxCfg.ConnConfig.ConnectTimeout = orDefaultDuration(cfg.ConnTimeout, 5*time.Second)

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: creating pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pgxCfg.ConnConfig.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: initial ping failed: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Check implements health.Checker
func (db *DB) Check(ctx context.Context) error {
	return db.Ping(ctx)
}

// WithTx runs fn inside a transaction: commits if fn returns nil, rolls
// back and returns fn's error otherwise. Also rolls back (and returns the
// original error) if fn panics, re-panicking after rollback so the panic
// still propagetes.
func (db *DB) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) (err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		err = tx.Commit(ctx)
	}()

	err = fn(ctx, tx)
	return err
}

func orDefault(v, def int32) int32 {
	if v <= 0 {
		return def
	}
	return v
}

func orDefaultDuration(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}

	return v
}
