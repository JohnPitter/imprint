package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// LedgerStore is the append-only token economy ledger (Phase 1). Every write is
// best-effort: a failure to record a spend or a saving must never break the main
// capture→compress→inject path (invariant 6), so the write methods swallow DB
// errors and return nil. The balance is always computed by aggregation at read
// time — we never read-modify-write a shared counter (invariant A4).
type LedgerStore struct {
	db *DB
}

func NewLedgerStore(db *DB) *LedgerStore {
	return &LedgerStore{db: db}
}

var ledgerIDSeq atomic.Int64

func ledgerID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), ledgerIDSeq.Add(1))
}

// SpendEntry is one background-LLM (Haiku) spend event.
type SpendEntry struct {
	SpendPoint   string // "compress" | "consolidate" | "graph_extract" | ...
	Provider     string
	SessionID    string
	Project      string // may be empty; resolved via session at read time
	InputTokens  int
	OutputTokens int
}

// AppendSpend records one Haiku spend event. Confidence is always "measured":
// these are real token counts reported by the provider.
func (s *LedgerStore) AppendSpend(e SpendEntry) {
	if s == nil || s.db == nil {
		return
	}
	if e.InputTokens == 0 && e.OutputTokens == 0 {
		return // nothing to account for (e.g. local backend without usage)
	}
	_, _ = s.db.Exec(`
		INSERT INTO token_ledger
			(id, kind, spend_point, provider, session_id, project, input_tokens, output_tokens, confidence)
		VALUES (?, 'haiku_spend', ?, ?, ?, ?, ?, ?, 'measured')`,
		ledgerID("spend"), e.SpendPoint, e.Provider, nullable(e.SessionID), e.Project,
		e.InputTokens, e.OutputTokens,
	)
}

// InjectionEntry is one memory/observation injected into a session's context.
type InjectionEntry struct {
	SessionID string
	Project   string
	Layer     string // "L0" | "L1" | "L2"
	ItemType  string // "memory" | "observation" | "summary"
	ItemID    string
	OccTokens int
	Files     []string
	Concepts  []string
}

// AppendInjection records one injected item. Returns the generated row id, which
// is used as the ref_id when the item is later credited as a saving.
func (s *LedgerStore) AppendInjection(e InjectionEntry) string {
	if s == nil || s.db == nil || e.SessionID == "" {
		return ""
	}
	id := ledgerID("inj")
	filesJSON, _ := json.Marshal(e.Files)
	conceptsJSON, _ := json.Marshal(e.Concepts)
	_, _ = s.db.Exec(`
		INSERT INTO injection_log
			(id, session_id, project, layer, item_type, item_id, occ_tokens, files, concepts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, e.SessionID, e.Project, e.Layer, e.ItemType, e.ItemID, e.OccTokens,
		string(filesJSON), string(conceptsJSON),
	)
	return id
}

// CreditUsage implements the "memory used" signal via cheap file/concept
// co-occurrence (decision 3.3): when a later observation in the same session
// touches the files or concepts of an injected item, that injection is credited
// as a saving. The credited amount is the item's own occupation tokens — a
// deliberately conservative floor on what re-exploration would have cost, since
// the raw source the memory compressed is always larger than the injected form
// (baseline decision 3.2.1, confidence = "proxy"). INSERT OR IGNORE on the
// unique ref_id index makes each injection credited at most once, atomically.
func (s *LedgerStore) CreditUsage(sessionID string, files, concepts []string) {
	if s == nil || s.db == nil || sessionID == "" {
		return
	}
	touch := make(map[string]struct{}, len(files)+len(concepts))
	for _, f := range files {
		touch[strings.ToLower(strings.TrimSpace(f))] = struct{}{}
	}
	for _, c := range concepts {
		touch[strings.ToLower(strings.TrimSpace(c))] = struct{}{}
	}
	delete(touch, "")
	if len(touch) == 0 {
		return
	}

	rows, err := s.db.Query(`
		SELECT id, project, occ_tokens, files, concepts
		  FROM injection_log
		 WHERE session_id = ?`, sessionID)
	if err != nil {
		return
	}
	defer rows.Close()

	type candidate struct {
		id      string
		project string
		occ     int
	}
	var hits []candidate
	for rows.Next() {
		var (
			id, project        string
			occ                int
			filesJSON, conJSON string
		)
		if err := rows.Scan(&id, &project, &occ, &filesJSON, &conJSON); err != nil {
			continue
		}
		if injectionTouched(filesJSON, conJSON, touch) {
			hits = append(hits, candidate{id: id, project: project, occ: occ})
		}
	}
	_ = rows.Err()

	for _, h := range hits {
		_, _ = s.db.Exec(`
			INSERT OR IGNORE INTO token_ledger
				(id, kind, session_id, project, ref_id, saved_tokens, confidence)
			VALUES (?, 'saving', ?, ?, ?, ?, 'proxy')`,
			ledgerID("save"), nullable(sessionID), h.project, h.id, h.occ,
		)
	}
}

// injectionTouched reports whether any file/concept of an injected item appears
// in the touch set of a later observation.
func injectionTouched(filesJSON, conceptsJSON string, touch map[string]struct{}) bool {
	var items []string
	_ = json.Unmarshal([]byte(filesJSON), &items)
	for _, it := range items {
		if _, ok := touch[strings.ToLower(strings.TrimSpace(it))]; ok {
			return true
		}
	}
	items = items[:0]
	_ = json.Unmarshal([]byte(conceptsJSON), &items)
	for _, it := range items {
		if _, ok := touch[strings.ToLower(strings.TrimSpace(it))]; ok {
			return true
		}
	}
	return false
}

// DayPoint is one day of the economy time series.
type DayPoint struct {
	Day         string `json:"day"`
	HaikuTokens int64  `json:"haikuTokens"`
	SavedTokens int64  `json:"savedTokens"`
	Saldo       int64  `json:"saldo"`
}

// EconomySummary is the aggregated token balance for a scope/window.
type EconomySummary struct {
	Project        string     `json:"project"`
	SinceDays      int        `json:"sinceDays"`
	HaikuInput     int64      `json:"haikuInputTokens"`
	HaikuOutput    int64      `json:"haikuOutputTokens"`
	HaikuTokens    int64      `json:"haikuTokens"`
	SavedTokens    int64      `json:"savedTokens"`
	InjectedTokens int64      `json:"injectedTokens"`
	Saldo          int64      `json:"saldoTokens"`
	SpendCalls     int64      `json:"spendCalls"`
	InjectionItems int64      `json:"injectionItems"`
	SavingEvents   int64      `json:"savingEvents"`
	UsedRatio      float64    `json:"usedRatio"`
	Confidence     string     `json:"confidence"`
	Daily          []DayPoint `json:"daily"`
}

// Economy aggregates the ledger for the given project (empty = all repos) over
// the given window (sinceDays <= 0 = all time). The saldo is contexto_poupado −
// haiku_gasto; it is shown honestly, including negative values on cold start.
func (s *LedgerStore) Economy(project string, sinceDays int) (EconomySummary, error) {
	out := EconomySummary{Project: project, SinceDays: sinceDays, Confidence: "proxy", Daily: []DayPoint{}}
	if s == nil || s.db == nil {
		return out, nil
	}

	// Effective project for a spend row: its own project, else the session's.
	const effProj = "COALESCE(NULLIF(l.project,''), s.project, '')"

	spendWhere, spendArgs := projectWindow(effProj, project, sinceDays)
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(l.input_tokens),0), COALESCE(SUM(l.output_tokens),0), COUNT(*)
		  FROM token_ledger l
		  LEFT JOIN sessions s ON s.id = l.session_id
		 WHERE l.kind = 'haiku_spend'`+spendWhere,
		spendArgs...,
	).Scan(&out.HaikuInput, &out.HaikuOutput, &out.SpendCalls)
	if err != nil {
		return out, fmt.Errorf("economy spend: %w", err)
	}
	out.HaikuTokens = out.HaikuInput + out.HaikuOutput

	saveWhere, saveArgs := projectWindow(effProj, project, sinceDays)
	if err := s.db.QueryRow(`
		SELECT COALESCE(SUM(l.saved_tokens),0), COUNT(*)
		  FROM token_ledger l
		  LEFT JOIN sessions s ON s.id = l.session_id
		 WHERE l.kind = 'saving'`+saveWhere,
		saveArgs...,
	).Scan(&out.SavedTokens, &out.SavingEvents); err != nil {
		return out, fmt.Errorf("economy saving: %w", err)
	}

	injWhere, injArgs := projectWindow("l.project", project, sinceDays)
	if err := s.db.QueryRow(`
		SELECT COALESCE(SUM(l.occ_tokens),0), COUNT(*)
		  FROM injection_log l
		 WHERE 1=1`+injWhere,
		injArgs...,
	).Scan(&out.InjectedTokens, &out.InjectionItems); err != nil {
		return out, fmt.Errorf("economy injection: %w", err)
	}

	out.Saldo = out.SavedTokens - out.HaikuTokens
	if out.InjectionItems > 0 {
		out.UsedRatio = float64(out.SavingEvents) / float64(out.InjectionItems)
	}

	daily, err := s.economyDaily(project, 14)
	if err == nil {
		out.Daily = daily
	}
	return out, nil
}

// economyDaily returns the last n days of haiku spend vs saved, oldest first.
func (s *LedgerStore) economyDaily(project string, n int) ([]DayPoint, error) {
	const effProj = "COALESCE(NULLIF(l.project,''), s.project, '')"
	byDay := map[string]*DayPoint{}

	spendWhere, spendArgs := projectWindow(effProj, project, 0)
	spendRows, err := s.db.Query(`
		SELECT date(l.ts) AS d, COALESCE(SUM(l.input_tokens + l.output_tokens),0)
		  FROM token_ledger l
		  LEFT JOIN sessions s ON s.id = l.session_id
		 WHERE l.kind = 'haiku_spend'`+spendWhere+` GROUP BY d`, spendArgs...)
	if err != nil {
		return nil, err
	}
	for spendRows.Next() {
		var d string
		var v int64
		if err := spendRows.Scan(&d, &v); err == nil {
			byDay[d] = &DayPoint{Day: d, HaikuTokens: v}
		}
	}
	spendRows.Close()

	saveWhere, saveArgs := projectWindow(effProj, project, 0)
	saveRows, err := s.db.Query(`
		SELECT date(l.ts) AS d, COALESCE(SUM(l.saved_tokens),0)
		  FROM token_ledger l
		  LEFT JOIN sessions s ON s.id = l.session_id
		 WHERE l.kind = 'saving'`+saveWhere+` GROUP BY d`, saveArgs...)
	if err != nil {
		return nil, err
	}
	for saveRows.Next() {
		var d string
		var v int64
		if err := saveRows.Scan(&d, &v); err == nil {
			p, ok := byDay[d]
			if !ok {
				p = &DayPoint{Day: d}
				byDay[d] = p
			}
			p.SavedTokens = v
		}
	}
	saveRows.Close()

	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	// date() yields YYYY-MM-DD, so lexical sort is chronological.
	for i := 1; i < len(days); i++ {
		for j := i; j > 0 && days[j-1] > days[j]; j-- {
			days[j-1], days[j] = days[j], days[j-1]
		}
	}
	if n > 0 && len(days) > n {
		days = days[len(days)-n:]
	}
	out := make([]DayPoint, 0, len(days))
	for _, d := range days {
		p := byDay[d]
		p.Saldo = p.SavedTokens - p.HaikuTokens
		out = append(out, *p)
	}
	return out, nil
}

// projectWindow builds the trailing WHERE fragment (with leading " AND ") and
// args for an optional project filter and an optional sinceDays window.
func projectWindow(projExpr, project string, sinceDays int) (string, []any) {
	var sb strings.Builder
	var args []any
	if project != "" {
		sb.WriteString(" AND " + projExpr + " = ?")
		args = append(args, project)
	}
	if sinceDays > 0 {
		sb.WriteString(" AND l.ts >= datetime('now', ?)")
		args = append(args, fmt.Sprintf("-%d days", sinceDays))
	}
	return sb.String(), args
}

// nullable returns nil for an empty string so it stores as SQL NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
