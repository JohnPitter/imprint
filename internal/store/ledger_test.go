package store

import "testing"

func newTestLedger(t *testing.T) *LedgerStore {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewLedgerStore(db)
}

func TestLedger_SpendInjectCreditEconomy(t *testing.T) {
	l := newTestLedger(t)

	// One Haiku spend attributed to repoA / session s1.
	l.AppendSpend(SpendEntry{
		SpendPoint: "compress", Provider: "anthropic",
		SessionID: "s1", Project: "repoA",
		InputTokens: 100, OutputTokens: 50,
	})

	// One injected memory carrying file a.go, occupying 30 tokens.
	injID := l.AppendInjection(InjectionEntry{
		SessionID: "s1", Project: "repoA", Layer: "L2", ItemType: "observation",
		ItemID: "cobs_1", OccTokens: 30, Files: []string{"a.go"},
	})
	if injID == "" {
		t.Fatal("AppendInjection returned empty id")
	}

	// A later observation touches a.go → the injection is credited as a saving.
	l.CreditUsage("s1", []string{"A.GO"}, nil) // case-insensitive match
	// Crediting again must be idempotent (unique ref_id).
	l.CreditUsage("s1", []string{"a.go"}, nil)

	got, err := l.Economy("repoA", 0)
	if err != nil {
		t.Fatalf("Economy() error: %v", err)
	}
	if got.HaikuTokens != 150 {
		t.Errorf("HaikuTokens = %d, want 150", got.HaikuTokens)
	}
	if got.SavedTokens != 30 {
		t.Errorf("SavedTokens = %d, want 30", got.SavedTokens)
	}
	if got.SavingEvents != 1 {
		t.Errorf("SavingEvents = %d, want 1 (credit must be idempotent)", got.SavingEvents)
	}
	if got.InjectionItems != 1 {
		t.Errorf("InjectionItems = %d, want 1", got.InjectionItems)
	}
	// Honest negative saldo: a single compress that saved little nets negative.
	if got.Saldo != -120 {
		t.Errorf("Saldo = %d, want -120 (30 saved − 150 haiku)", got.Saldo)
	}
	if got.UsedRatio != 1.0 {
		t.Errorf("UsedRatio = %v, want 1.0", got.UsedRatio)
	}
}

func TestLedger_ScopedByProject(t *testing.T) {
	l := newTestLedger(t)
	l.AppendSpend(SpendEntry{SpendPoint: "compress", SessionID: "s1", Project: "repoA", InputTokens: 10, OutputTokens: 5})

	other, err := l.Economy("repoB", 0)
	if err != nil {
		t.Fatalf("Economy() error: %v", err)
	}
	if other.HaikuTokens != 0 || other.SpendCalls != 0 {
		t.Errorf("repoB leaked repoA spend: haiku=%d calls=%d", other.HaikuTokens, other.SpendCalls)
	}
}

func TestLedger_SpendZeroTokensIgnored(t *testing.T) {
	l := newTestLedger(t)
	l.AppendSpend(SpendEntry{SpendPoint: "compress", Project: "repoA"}) // 0/0 tokens

	got, err := l.Economy("repoA", 0)
	if err != nil {
		t.Fatalf("Economy() error: %v", err)
	}
	if got.SpendCalls != 0 {
		t.Errorf("SpendCalls = %d, want 0 (zero-token spend must be skipped)", got.SpendCalls)
	}
}

func TestLedger_NilSafe(t *testing.T) {
	var l *LedgerStore
	// None of these should panic on a nil store (degradação graciosa).
	l.AppendSpend(SpendEntry{InputTokens: 1})
	l.CreditUsage("s1", []string{"a.go"}, nil)
	if id := l.AppendInjection(InjectionEntry{SessionID: "s1"}); id != "" {
		t.Errorf("nil store AppendInjection = %q, want empty", id)
	}
	if got, err := l.Economy("", 0); err != nil || got.HaikuTokens != 0 {
		t.Errorf("nil store Economy = %+v, %v", got, err)
	}
}
