package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDatabase(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	dbPath := filepath.Join(dir, "imprint.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected database file at %s, but it does not exist", dbPath)
	}
}

func TestOpen_RunsMigrations(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	expectedTables := []string{
		"sessions",
		"memories",
		"compressed_observations",
		"graph_nodes",
		"graph_edges",
		"raw_observations",
		"semantic_memories",
		"procedural_memories",
		"session_summaries",
		"actions",
		"action_edges",
		"leases",
		"routines",
		"signals",
		"checkpoints",
		"sentinels",
		"sketches",
		"crystals",
		"lessons",
		"insights",
		"facets",
		"audit_log",
		"project_profiles",
		"mesh_peers",
		"embeddings",
		"access_log",
		"snapshots",
		"dedup_cache",
		"_migrations",
	}

	for _, table := range expectedTables {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)
		if err != nil {
			t.Errorf("query sqlite_master for table %q: %v", table, err)
			continue
		}
		if count == 0 {
			t.Errorf("expected table %q to exist after migrations, but it was not found", table)
		}
	}
}

func TestOpen_MigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open() error: %v", err)
	}
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("second Open() error: %v", err)
	}
	defer db2.Close()

	// Verify the migration was recorded exactly once.
	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = '001_initial.sql'").Scan(&count)
	if err != nil {
		t.Fatalf("query _migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration record for 001_initial.sql, got %d", count)
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}
