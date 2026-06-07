package llm

import (
	"errors"
	"sync"
	"time"
)

// ErrBudgetExceeded is returned by an instrumented Complete call when the Haiku
// token ceiling for the session or the day has been reached. It is NOT a
// provider failure: callers (the background pipeline) log it and continue, the
// raw observation is already stored, and the main path stays intact (invariant
// 6). It must not trip the circuit breaker.
var ErrBudgetExceeded = errors.New("imprint: haiku token budget exceeded")

// nowDay is the clock used for day rollover. Overridable in tests.
var nowDay = func() string { return time.Now().UTC().Format("2006-01-02") }

// BudgetGate is the spend ceiling that protects *before* tokens are burned,
// complementing the ledger meter that measures *after*. It tracks cumulative
// Haiku tokens per session and per UTC day and denies further instrumented
// calls once a configured cap is reached. Concurrency-safe; spend accounting is
// additive so concurrent sessions can record without corrupting the count.
//
// This extends the existing LLM resilience gate (circuit breaker) with a second
// dimension — tokens — rather than introducing a separate spend pipeline.
type BudgetGate struct {
	mu             sync.Mutex
	perSession     int64 // 0 = unlimited
	perDay         int64 // 0 = unlimited
	day            string
	daySpent       int64
	sessionSpent   map[string]int64
	pausedSessions map[string]bool // sessions currently over budget (for status/UI)
}

// GlobalBudget is the process-wide gate. main.go configures its limits from
// config; the LLM layer consults it before every instrumented call.
var GlobalBudget = &BudgetGate{
	sessionSpent:   map[string]int64{},
	pausedSessions: map[string]bool{},
}

// SetLimits configures the per-session and per-day Haiku token ceilings.
// 0 disables that dimension. Safe to call at boot before any traffic.
func (b *BudgetGate) SetLimits(perSession, perDay int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.perSession = int64(perSession)
	b.perDay = int64(perDay)
}

// rolloverLocked resets the daily counter when the UTC day changes. Caller holds b.mu.
func (b *BudgetGate) rolloverLocked() {
	d := nowDay()
	if d != b.day {
		b.day = d
		b.daySpent = 0
	}
}

// Allow reports whether an instrumented call for sessionID may proceed. It
// returns false once either the session or the day cap is reached, which makes
// the caller skip the LLM call gracefully.
func (b *BudgetGate) Allow(sessionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverLocked()
	if b.perDay > 0 && b.daySpent >= b.perDay {
		return false
	}
	if b.perSession > 0 && sessionID != "" && b.sessionSpent[sessionID] >= b.perSession {
		b.pausedSessions[sessionID] = true
		return false
	}
	return true
}

// Record adds a completed call's tokens to the session and day counters.
func (b *BudgetGate) Record(sessionID string, tokens int) {
	if tokens <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverLocked()
	b.daySpent += int64(tokens)
	if sessionID != "" {
		b.sessionSpent[sessionID] += int64(tokens)
		if b.perSession > 0 && b.sessionSpent[sessionID] >= b.perSession {
			b.pausedSessions[sessionID] = true
		}
	}
}

// BudgetStatus is a read-only snapshot of the gate for the economy endpoint/UI.
type BudgetStatus struct {
	PerSessionLimit int64  `json:"perSessionLimit"`
	PerDayLimit     int64  `json:"perDayLimit"`
	DaySpent        int64  `json:"daySpent"`
	Day             string `json:"day"`
	PausedSessions  int    `json:"pausedSessions"`
	DayExceeded     bool   `json:"dayExceeded"`
}

// Status returns a snapshot for display. Cheap; safe to call per request.
func (b *BudgetGate) Status() BudgetStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverLocked()
	return BudgetStatus{
		PerSessionLimit: b.perSession,
		PerDayLimit:     b.perDay,
		DaySpent:        b.daySpent,
		Day:             b.day,
		PausedSessions:  len(b.pausedSessions),
		DayExceeded:     b.perDay > 0 && b.daySpent >= b.perDay,
	}
}

// SessionPaused reports whether a given session has hit its cap. Used by the
// injection path to fall back to the minimum when the budget is spent.
func (b *BudgetGate) SessionPaused(sessionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverLocked()
	if b.perDay > 0 && b.daySpent >= b.perDay {
		return true
	}
	if b.perSession > 0 && sessionID != "" && b.sessionSpent[sessionID] >= b.perSession {
		return true
	}
	return false
}
