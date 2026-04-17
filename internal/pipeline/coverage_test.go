package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"imprint/internal/store"
)

// ---------------------------------------------------------------------------
// heuristic.go — ClassifyMemoryType / containsAny
// ---------------------------------------------------------------------------

func TestClassifyMemoryType(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"decision", "We decided to use Postgres over MySQL", "decision"},
		{"decision_chose", "We chose fiber for routing", "decision"},
		{"preference", "I prefer tabs over spaces", "preference"},
		{"preference_always", "always use context.Background for tests", "preference"},
		{"bug", "fix: crash on nil pointer", "bug"},
		{"bug_error", "this error keeps showing up", "bug"},
		{"architecture", "the architecture uses a layered module structure", "architecture"},
		{"workflow", "steps to deploy the release via ci/cd", "workflow"},
		{"pattern", "when the cache is empty, the loader realized that it returns nil", "pattern"},
		{"fact_default", "The quick brown fox", "fact"},
		{"case_insensitive", "DECIDED TO migrate", "decision"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyMemoryType(tc.in); got != tc.want {
				t.Fatalf("ClassifyMemoryType(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", "foo", "world") {
		t.Fatal("expected match for 'world'")
	}
	if containsAny("hello", "foo", "bar") {
		t.Fatal("unexpected match")
	}
	if containsAny("anything") {
		t.Fatal("no patterns should return false")
	}
}

// ---------------------------------------------------------------------------
// xml.go — splitXMLBlocks
// ---------------------------------------------------------------------------

func TestSplitXMLBlocks(t *testing.T) {
	in := `<a>1</a><a>2</a><a>three</a>`
	got := splitXMLBlocks(in, "a")
	if len(got) != 3 {
		t.Fatalf("expected 3 blocks, got %d: %v", len(got), got)
	}
	if got[0] != "<a>1</a>" || got[2] != "<a>three</a>" {
		t.Fatalf("unexpected blocks: %v", got)
	}
}

func TestSplitXMLBlocks_None(t *testing.T) {
	if got := splitXMLBlocks("no tags here", "x"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSplitXMLBlocks_Multiline(t *testing.T) {
	in := "<m>\nhello\n</m><m>world</m>"
	got := splitXMLBlocks(in, "m")
	if len(got) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// pattern.go — PatternDetector and helpers
// ---------------------------------------------------------------------------

func makeObs(typ, title string, files, concepts []string) store.CompressedObservationRow {
	return store.CompressedObservationRow{
		Type:     typ,
		Title:    title,
		Files:    files,
		Concepts: concepts,
	}
}

func TestMinFloat(t *testing.T) {
	if minFloat(1.0, 2.0) != 1.0 {
		t.Fatal("min failed")
	}
	if minFloat(3.0, 2.0) != 2.0 {
		t.Fatal("min failed")
	}
	if minFloat(5.0, 5.0) != 5.0 {
		t.Fatal("equal failed")
	}
}

func TestAppendUnique(t *testing.T) {
	got := appendUnique([]string{"a", "b"}, "b", "c", "a", "d")
	want := []string{"a", "b", "c", "d"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}

	got = appendUnique(nil, "x", "x", "y")
	if strings.Join(got, ",") != "x,y" {
		t.Fatalf("nil-seed: got %v", got)
	}
}

func TestPatternDetector_DetectPatterns(t *testing.T) {
	pd := NewPatternDetector()
	// Build enough data to trigger all detectors.
	obs := []store.CompressedObservationRow{}
	// Co-change: "a.go" + "b.go" paired 3 times -> triggers co_change.
	for range 3 {
		obs = append(obs, makeObs("file_operation", "edit", []string{"a.go", "b.go"}, []string{"go"}))
	}
	// Error repeats: same error title twice -> triggers error_repeat.
	obs = append(obs, makeObs("error", "nil pointer", []string{"x.go"}, []string{"err"}))
	obs = append(obs, makeObs("error", "nil pointer", []string{"x.go"}, []string{"err"}))
	// Non-error with same title shouldn't count.
	obs = append(obs, makeObs("file_operation", "nil pointer", nil, nil))
	// Frequent file: "hot.go" appears in 5 obs.
	for range 5 {
		obs = append(obs, makeObs("file_operation", "t", []string{"hot.go"}, []string{"hotconcept"}))
	}

	patterns := pd.DetectPatterns(obs)

	seen := map[string]bool{}
	for _, p := range patterns {
		seen[p.Type] = true
		if p.Confidence <= 0 || p.Confidence > 1 {
			t.Fatalf("confidence out of range for %s: %f", p.Type, p.Confidence)
		}
		if p.Frequency <= 0 {
			t.Fatalf("zero freq for %s", p.Type)
		}
	}
	for _, typ := range []string{"co_change", "error_repeat", "frequent_file", "frequent_concept"} {
		if !seen[typ] {
			t.Fatalf("expected to detect %s in patterns, got types: %v", typ, seen)
		}
	}
}

func TestPatternDetector_BelowThreshold(t *testing.T) {
	pd := NewPatternDetector()
	obs := []store.CompressedObservationRow{
		makeObs("error", "rare", []string{"f.go"}, []string{"c"}),       // only 1 error
		makeObs("file_operation", "x", []string{"a.go", "b.go"}, nil),   // only 1 pair
		makeObs("file_operation", "y", []string{"only.go"}, []string{}), // 1 file
	}
	patterns := pd.DetectPatterns(obs)
	if len(patterns) != 0 {
		t.Fatalf("expected no patterns below threshold, got %v", patterns)
	}
}

func TestPatternDetector_CoChange_KeyOrdering(t *testing.T) {
	pd := NewPatternDetector()
	// Ensure pairs with reversed file orders are grouped.
	obs := []store.CompressedObservationRow{
		makeObs("file_operation", "edit", []string{"z.go", "a.go"}, nil),
		makeObs("file_operation", "edit", []string{"a.go", "z.go"}, nil),
		makeObs("file_operation", "edit", []string{"z.go", "a.go"}, nil),
	}
	patterns := pd.detectCoChanges(obs)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 co_change pattern (canonical key), got %d: %v", len(patterns), patterns)
	}
	if patterns[0].Frequency != 3 {
		t.Fatalf("expected frequency 3, got %d", patterns[0].Frequency)
	}
}

// ---------------------------------------------------------------------------
// reflect.go — parseFloat, parseReflectedInsights, Reflect
// ---------------------------------------------------------------------------

func TestParseFloat(t *testing.T) {
	if parseFloat("0.75", 0.5) != 0.75 {
		t.Fatal("parse ok")
	}
	if parseFloat("  0.5  ", 0.1) != 0.5 {
		t.Fatal("trim")
	}
	if parseFloat("", 0.42) != 0.42 {
		t.Fatal("empty default")
	}
	if parseFloat("not-a-number", 0.3) != 0.3 {
		t.Fatal("invalid default")
	}
}

func TestParseReflectedInsights(t *testing.T) {
	resp := `<insights>
<insight>
  <title>First insight</title>
  <content>Something important</content>
  <confidence>0.8</confidence>
  <concepts><concept>go</concept><concept>testing</concept></concepts>
</insight>
<insight>
  <title>Clamped high</title>
  <content>Clamp me</content>
  <confidence>5.0</confidence>
</insight>
<insight>
  <title>Clamped low</title>
  <content>Clamp me too</content>
  <confidence>-0.5</confidence>
</insight>
<insight>
  <content>No title, should be dropped</content>
</insight>
</insights>`
	got := parseReflectedInsights(resp)
	if len(got) != 3 {
		t.Fatalf("expected 3 insights, got %d", len(got))
	}
	if got[0].Title != "First insight" || got[0].Confidence != 0.8 {
		t.Fatalf("first insight wrong: %+v", got[0])
	}
	if len(got[0].Concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %v", got[0].Concepts)
	}
	if got[1].Confidence != 1.0 {
		t.Fatalf("expected clamp to 1.0, got %f", got[1].Confidence)
	}
	if got[2].Confidence != 0.0 {
		t.Fatalf("expected clamp to 0.0, got %f", got[2].Confidence)
	}
}

func TestParseReflectedInsights_NoWrapper(t *testing.T) {
	// If no <insights> wrapper, falls back to scanning the whole response.
	resp := `<insight><title>Loose</title><content>data</content><confidence>0.5</confidence></insight>`
	got := parseReflectedInsights(resp)
	if len(got) != 1 || got[0].Title != "Loose" {
		t.Fatalf("fallback failed: %v", got)
	}
}

func TestReflector_Empty(t *testing.T) {
	r := NewReflector(&mockLLMProvider{response: ""})
	got, err := r.Reflect(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil insights, got %v", got)
	}
}

func TestReflector_WithData(t *testing.T) {
	resp := `<insights><insight><title>T</title><content>C</content><confidence>0.9</confidence></insight></insights>`
	mock := &mockLLMProvider{response: resp}
	r := NewReflector(mock)
	narr := "did stuff"
	memories := []store.MemoryRow{
		{ID: "m1", Type: "pattern", Title: "M1", Content: strings.Repeat("x", 300), Strength: 7},
	}
	observations := []store.CompressedObservationRow{
		{ID: "o1", Type: "file_operation", Title: "edit", Narrative: &narr, Concepts: []string{"go"}},
	}
	got, err := r.Reflect(context.Background(), memories, observations)
	if err != nil {
		t.Fatalf("Reflect error: %v", err)
	}
	if len(got) != 1 || got[0].Title != "T" {
		t.Fatalf("unexpected insights: %v", got)
	}
	if mock.calls.Load() != 1 {
		t.Fatalf("expected 1 LLM call, got %d", mock.calls.Load())
	}
}

func TestReflector_LLMError(t *testing.T) {
	r := NewReflector(&mockLLMProvider{err: errors.New("boom")})
	_, err := r.Reflect(context.Background(), []store.MemoryRow{{ID: "m"}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// consolidate.go — groupBySharedConcepts, parseConsolidatedMemories, Consolidate
// ---------------------------------------------------------------------------

func TestGroupBySharedConcepts(t *testing.T) {
	obs := []store.CompressedObservationRow{
		{ID: "1", Concepts: []string{"go", "http"}},
		{ID: "2", Concepts: []string{"http"}},         // joins with 1 via "http"
		{ID: "3", Concepts: []string{"react"}},        // isolated
		{ID: "4", Concepts: []string{"react", "css"}}, // joins with 3 via "react"
		{ID: "5", Concepts: []string{"go"}},           // joins with 1 via "go"
	}
	groups := groupBySharedConcepts(obs)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	// Verify total count.
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	if total != 5 {
		t.Fatalf("expected 5 total obs in groups, got %d", total)
	}
}

func TestGroupBySharedConcepts_NoConcepts(t *testing.T) {
	obs := []store.CompressedObservationRow{
		{ID: "1"}, {ID: "2"}, {ID: "3"},
	}
	groups := groupBySharedConcepts(obs)
	// Each observation without shared concepts becomes its own group.
	if len(groups) != 3 {
		t.Fatalf("expected 3 isolated groups, got %d", len(groups))
	}
}

func TestParseConsolidatedMemories(t *testing.T) {
	resp := `<memories>
<memory>
  <type>pattern</type>
  <title>Use context</title>
  <content>Always pass ctx</content>
  <concepts><concept>go</concept></concepts>
  <files><file>main.go</file></files>
  <strength>8</strength>
</memory>
<memory>
  <title>Low strength clamp</title>
  <content>Tiny</content>
  <strength>0</strength>
</memory>
<memory>
  <title>High strength clamp</title>
  <content>Big</content>
  <strength>99</strength>
</memory>
<memory>
  <content>no title, should drop</content>
</memory>
</memories>`
	got := parseConsolidatedMemories(resp)
	if len(got) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(got))
	}
	if got[0].Type != "pattern" || got[0].Strength != 8 {
		t.Fatalf("first memory wrong: %+v", got[0])
	}
	if got[1].Strength != 5 { // defaulted from < 1
		t.Fatalf("expected strength clamped to 5, got %d", got[1].Strength)
	}
	if got[2].Strength != 10 {
		t.Fatalf("expected strength clamped to 10, got %d", got[2].Strength)
	}
	// Memory without explicit <type> should be heuristic-classified.
	if got[1].Type == "" {
		t.Fatalf("expected heuristic type, got empty")
	}
}

func TestParseConsolidatedMemories_NoWrapper(t *testing.T) {
	resp := `<memory><type>fact</type><title>A</title><content>B</content></memory>`
	got := parseConsolidatedMemories(resp)
	if len(got) != 1 || got[0].Title != "A" {
		t.Fatalf("fallback failed: %v", got)
	}
}

func TestConsolidator_Empty(t *testing.T) {
	c := NewConsolidator(&mockLLMProvider{})
	got, err := c.Consolidate(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestConsolidator_Consolidate(t *testing.T) {
	resp := `<memories><memory><type>pattern</type><title>X</title><content>Y</content><strength>6</strength></memory></memories>`
	mock := &mockLLMProvider{response: resp}
	c := NewConsolidator(mock)
	narr := "narr"
	obs := []store.CompressedObservationRow{
		{ID: "1", Type: "file_operation", Title: "Edit a", Narrative: &narr, Concepts: []string{"go"}, Files: []string{"a.go"}},
		{ID: "2", Type: "file_operation", Title: "Edit b", Concepts: []string{"go"}, Files: []string{"b.go"}},
	}
	got, err := c.Consolidate(context.Background(), obs)
	if err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(got))
	}
	if got[0].Title != "X" || got[0].Strength != 6 {
		t.Fatalf("unexpected memory: %+v", got[0])
	}
	if mock.calls.Load() != 1 {
		t.Fatalf("expected 1 LLM call (1 group), got %d", mock.calls.Load())
	}
}

func TestConsolidator_LLMError(t *testing.T) {
	c := NewConsolidator(&mockLLMProvider{err: errors.New("nope")})
	obs := []store.CompressedObservationRow{
		{ID: "1", Concepts: []string{"go"}},
		{ID: "2", Concepts: []string{"go"}},
	}
	_, err := c.Consolidate(context.Background(), obs)
	if err == nil {
		t.Fatal("expected error propagated from LLM")
	}
}

func TestConsolidator_CapsGroupSize(t *testing.T) {
	// Cap each group at 8 observations. Build 10 with same concept.
	resp := `<memories><memory><type>fact</type><title>T</title><content>C</content></memory></memories>`
	mock := &mockLLMProvider{response: resp}
	c := NewConsolidator(mock)
	obs := make([]store.CompressedObservationRow, 10)
	for i := range obs {
		obs[i] = store.CompressedObservationRow{
			ID:       string(rune('a' + i)),
			Concepts: []string{"shared"},
		}
	}
	got, err := c.Consolidate(context.Background(), obs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(got))
	}
}
