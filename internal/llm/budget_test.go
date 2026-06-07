package llm

import "testing"

func newGate(perSession, perDay int) *BudgetGate {
	g := &BudgetGate{sessionSpent: map[string]int64{}, pausedSessions: map[string]bool{}}
	g.SetLimits(perSession, perDay)
	return g
}

func TestBudget_SessionCap(t *testing.T) {
	g := newGate(100, 0)
	if !g.Allow("s1") {
		t.Fatal("expected allow before any spend")
	}
	g.Record("s1", 60)
	if !g.Allow("s1") {
		t.Fatal("expected allow at 60/100")
	}
	g.Record("s1", 40) // now at 100
	if g.Allow("s1") {
		t.Error("expected deny at 100/100 (cap reached)")
	}
	if !g.SessionPaused("s1") {
		t.Error("expected SessionPaused true at cap")
	}
	// A different session is unaffected.
	if !g.Allow("s2") {
		t.Error("expected other session still allowed")
	}
}

func TestBudget_DayCap(t *testing.T) {
	g := newGate(0, 150)
	g.Record("s1", 100)
	g.Record("s2", 60) // day total 160 >= 150
	if g.Allow("s1") {
		t.Error("expected deny once day cap reached (across sessions)")
	}
	if !g.Status().DayExceeded {
		t.Error("expected DayExceeded true")
	}
}

func TestBudget_DayRollover(t *testing.T) {
	orig := nowDay
	defer func() { nowDay = orig }()
	nowDay = func() string { return "2026-01-01" }

	g := newGate(0, 100)
	g.Record("s1", 100)
	if g.Allow("s1") {
		t.Fatal("expected deny at day cap")
	}
	nowDay = func() string { return "2026-01-02" } // new day
	if !g.Allow("s1") {
		t.Error("expected allow after day rollover (daily counter reset)")
	}
}

func TestBudget_Unlimited(t *testing.T) {
	g := newGate(0, 0) // both unlimited
	g.Record("s1", 10_000_000)
	if !g.Allow("s1") {
		t.Error("expected allow when both caps are 0 (unlimited)")
	}
}
