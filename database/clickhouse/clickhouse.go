// Package clickhouse wraps clickhouse-go/v2's native driver for
// high-volume event/analytics storage - the typical "many writes, large
// range-can reads, no per-row transactions" workload that doesn't belong
// in Postgres (see database/postgres for the OLTP side of a service)
package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config configures New
type Config struct {
	Addr     []string // e.g. []string{"clickhouse.db.svc:9000"}
	Database string
	Username string
	Password string
	Secure   bool // true to dial with TLS (ClickHouse cloud, TLS-terminated in-cluseter)

	DialTimeout  time.Duration // default 5s
	MaxOpenConns int           // default 10
	MaxIdleConns int           // default 5
}

// DB wraps driver.Conn (the native ClickHouse protocol interface - faster
// than the database/sql wrapper, and this SDK has no code depending on
// database/sql generically, so there's no reason to pay that overhead)
type DB struct {
	driver.Conn
}

// New opens a connection pool and verifies connectivity with Ping
func New(ctx context.Context, cfg Config) (*DB, error) {
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}

	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 10
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}

	opts := &clickhouse.Options{
		Addr: cfg.Addr,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout:  dialTimeout,
		MaxOpenConns: maxOpen,
		MaxIdleConns: maxIdle,
	}
	if cfg.Secure {
		opts.TLS = &tls.Config{}
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: opening connection: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("clickhouse: initial ping failed: %w", err)
	}

	return &DB{Conn: conn}, nil
}

// Check implements health.Checker
func (db *DB) Check(ctx context.Context) error {
	return db.Ping(ctx)
}

func (db *DB) BatchInsert(ctx context.Context, query string, fill func(driver.Batch) error) error {
	batch, err := db.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("clickhouse: preparing batch for %q: %w", query, err)
	}
	if err := fill(batch); err != nil {
		return fmt.Errorf("clickhouse: filling batch: %w", err)
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("clickhouse: sending batch: %w", err)
	}

	return nil
}
