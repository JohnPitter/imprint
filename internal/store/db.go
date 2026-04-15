package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// DB wraps *sql.DB with helper methods and migration support.
type DB struct {
	*sql.DB
	dataDir string
}

// Open creates or opens the SQLite database at dataDir/imprint.db,
// configures WAL mode, busy timeout, foreign keys, and runs pending migrations.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "imprint.db")
	dsn := fmt.Sprintf(
		"%s?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(normal)",
		dbPath,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	d := &DB{DB: db, dataDir: dataDir}

	if err := d.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}

// DataDir returns the directory containing the database file.
func (d *DB) DataDir() string {
	return d.dataDir
}

// runMigrations applies all pending SQL migrations from the embedded migrations/ directory.
// Each migration runs inside its own transaction. Applied migrations are tracked in the
// _migrations table so they are never re-applied.
func (d *DB) runMigrations() error {
	_, err := d.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			name       TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migration directory: %w", err)
	}

	// Collect and sort migration file names for deterministic ordering.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		// Check whether this migration has already been applied.
		var exists int
		if err := d.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = ?", name).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}

		content, err := fs.ReadFile(migrationFS, filepath.ToSlash(filepath.Join("migrations", name)))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := d.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction for %s: %w", name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO _migrations (name, applied_at) VALUES (?, ?)",
			name, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}
