package store

import "testing"

func TestEvalStore_AppendAndList(t *testing.T) {
	db := setupTestDB(t)
	store := NewEvalStore(db)

	sid := "ses-1"
	for _, c := range []EvalCandidate{
		{Source: "mcp", Operation: "search", QueryText: "auth flow", ReturnedIDs: []string{"m1", "m2"}, ResultCount: 2, SessionID: &sid},
		{Source: "http", Operation: "recall", QueryText: "previous decisions", ReturnedIDs: []string{"m3"}, ResultCount: 1},
		{Source: "mcp", Operation: "search", QueryText: "", ReturnedIDs: nil}, // empty query — skipped
	} {
		if err := store.Append(c); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	got, err := store.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 candidates (empty query was skipped), got %d", len(got))
	}

	n, err := store.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("expected count=2, got %d", n)
	}

	// Most recent first.
	if got[0].QueryText == got[1].QueryText {
		t.Errorf("expected distinct query texts, got duplicates")
	}
	if len(got[0].ReturnedIDs) == 0 {
		t.Errorf("expected returned ids to round-trip via JSON, got empty")
	}
}
