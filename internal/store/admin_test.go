package store

import (
	"testing"
	"time"
)

func seedProject(t *testing.T, db *DB, project, sid string) {
	t.Helper()
	ss := NewSessionStore(db)
	if err := ss.Create(&SessionRow{ID: sid, Project: project, StartedAt: time.Now(), Status: "active"}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	ms := NewMemoryStore(db)
	if err := ms.Create(&MemoryRow{
		ID: "mem_" + sid, Type: "architecture", Title: "t", Content: "c",
		SessionIDs: jsonRaw(sid), Strength: 7, IsLatest: 1,
	}); err != nil {
		t.Fatalf("create memory: %v", err)
	}
	is := NewIntuitionStore(db)
	if err := is.Create(&IntuitionRow{ID: "intu_" + sid, Project: project, Statement: "s"}); err != nil {
		t.Fatalf("create intuition: %v", err)
	}
}

func jsonRaw(items ...string) []byte {
	b := []byte(`["`)
	for i, it := range items {
		if i > 0 {
			b = append(b, []byte(`","`)...)
		}
		b = append(b, []byte(it)...)
	}
	b = append(b, []byte(`"]`)...)
	return b
}

func TestAdmin_PurgeProjectIsolated(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	seedProject(t, db, "p1", "s1")
	seedProject(t, db, "p2", "s2")

	admin := NewAdminStore(db)
	counts, err := admin.PurgeProject("p1")
	if err != nil {
		t.Fatalf("PurgeProject: %v", err)
	}
	if counts["sessions"] != 1 || counts["intuitions"] != 1 || counts["memories"] != 1 {
		t.Errorf("unexpected purge counts: %+v", counts)
	}

	// p1 gone.
	is := NewIntuitionStore(db)
	if a, _ := is.ListAll("p1", 10); len(a) != 0 {
		t.Errorf("expected p1 intuitions gone, got %d", len(a))
	}
	// p2 intact (isolation A1).
	if a, _ := is.ListAll("p2", 10); len(a) != 1 {
		t.Errorf("expected p2 intuition intact, got %d", len(a))
	}
	ms := NewMemoryStore(db)
	if m, _ := ms.ListRefinedByProject("p2", 0, 10); len(m) != 1 {
		t.Errorf("expected p2 memory intact, got %d", len(m))
	}
	if m, _ := ms.ListRefinedByProject("p1", 0, 10); len(m) != 0 {
		t.Errorf("expected p1 memory gone, got %d", len(m))
	}
}

func TestAdmin_ResetAll(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedProject(t, db, "p1", "s1")

	admin := NewAdminStore(db)
	if err := admin.ResetAll(); err != nil {
		t.Fatalf("ResetAll: %v", err)
	}
	is := NewIntuitionStore(db)
	if a, _ := is.ListAll("", 10); len(a) != 0 {
		t.Errorf("expected all intuitions gone after reset, got %d", len(a))
	}
}
