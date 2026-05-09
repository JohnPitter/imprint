package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMemoryStore_DecayOld(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	mk := func(id string, strength int, ageDays int) {
		mem := &MemoryRow{
			ID:                   id,
			Type:                 "fact",
			Title:                id,
			Content:              "x",
			Concepts:             json.RawMessage(`[]`),
			Files:                json.RawMessage(`[]`),
			SessionIDs:           json.RawMessage(`[]`),
			Strength:             strength,
			Version:              1,
			SourceObservationIDs: json.RawMessage(`[]`),
			IsLatest:             1,
		}
		if err := store.Create(mem); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		// Backdate created_at via direct UPDATE — Create() always uses now().
		oldTS := TimeToString(time.Now().AddDate(0, 0, -ageDays))
		if _, err := db.Exec(`UPDATE memories SET created_at = ? WHERE id = ?`, oldTS, id); err != nil {
			t.Fatalf("backdate %s: %v", id, err)
		}
	}

	// Should decay: low strength (≤3), old (>30d).
	mk("decay-1", 2, 45)
	mk("decay-2", 3, 60)
	// Should survive: low strength but recent.
	mk("keep-recent", 2, 10)
	// Should survive: old but high strength.
	mk("keep-strong", 8, 90)

	n, err := store.DecayOld(3, 30)
	if err != nil {
		t.Fatalf("DecayOld: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 decays, got %d", n)
	}

	// Verify which rows survived as latest.
	cases := []struct {
		id           string
		wantIsLatest int
	}{
		{"decay-1", 0},
		{"decay-2", 0},
		{"keep-recent", 1},
		{"keep-strong", 1},
	}
	for _, c := range cases {
		var isLatest int
		if err := db.QueryRow(`SELECT is_latest FROM memories WHERE id = ?`, c.id).Scan(&isLatest); err != nil {
			t.Fatalf("scan %s: %v", c.id, err)
		}
		if isLatest != c.wantIsLatest {
			t.Errorf("%s: is_latest=%d, want %d", c.id, isLatest, c.wantIsLatest)
		}
	}
}
