// Package migrate wraps golang-migrate so schema migrations run as an
// explicit step (typically a k8s Job run berfore the Deployment rollout
// in the GitOps pipeline - "migrate, then roll out the new image") rather
// than implicity on service startup, which causes races when multiple
// replicas start concurrently
//
// Migration files are plain SQL, named
// {version}_{description}.up.sql / .down.sql, e.g.:
//
// migations/
//
//	0001_init.up.sql
//	0001_init.down.sql
//	0002_add_status_to_issues.up.sql
//	0002_add_status_to_issues.down.sql
package migrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/database/postgres"
)

// Driver selects which database/{driver} golang-migrate backend to use
type Driver string

const (
	DriverPostgres   Driver = "postgres"
	DriverClickHouse Driver = "clickhouse"
)

// Up applies every pending migration found in sourceDir (a
// "file://..." path is built from it automatically) against dsn,
// using driver's native migration-tracking table (schema_migrations for
// Postgres, a ClickHouse table of the same name for ClickHouse)
func Up(driver Driver, sourceDir, dsn string) error {
	m, err := newMigrator(driver, sourceDir, dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}

// Down rolls back every applied migration. Mainly useful for local dev
// resets and integration tests - running this against a real environment
// is rarely what you want
func Down(driver Driver, sourceDir, dsn string) error {
	m, err := newMigrator(driver, sourceDir, dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: down: %w", err)
	}
	return nil
}

// Version reports the currently applied migration version and whether
// the database is in dirty state (a previous migration failed
// partway through and needs manual attention befor Up/Down will run
// again)
func Version(driver Driver, sourceDir, dsn string) (version uint, dirty bool, err error) {
	m, err := newMigrator(driver, sourceDir, dsn)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	return m.Version()
}

func newMigrator(driver Driver, sourceDir, dsn string) (*migrate.Migrate, error) {
	switch driver {
	case DriverPostgres:
		return migrate.New("file://"+sourceDir, ensureScheme(dsn, "postgres"))
	case DriverClickHouse:
		return migrate.New("file://"+sourceDir, ensureScheme(dsn, "clickhouse"))
	default:
		return nil, fmt.Errorf("migrate: unknown driver %q", driver)
	}
}

// ensureScheme is small guard so callers can pass a bare DSN (as used
// elsewhere in this SDK, e.g. config.PostgresConfig.DSN) without needing
// to know golang-migrate wants a URL with the driver name as scheme
func ensureScheme(dsn, scheme string) string {
	for i := 0; i < len(dsn)-2; i++ {
		if dsn[i] == ':' && dsn[i+1] == '/' && dsn[i+2] == '/' {
			return dsn // already has a scheme
		}
	}
	return scheme + "://" + dsn
}

// silence unused-import complaints for the driver packages, which are
// imported for their side-effecting init() (database driver registration)
// rather than called directly
var (
	_ = postgres.Config{}
	_ = clickhouse.Config{}
)
