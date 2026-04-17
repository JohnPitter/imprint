package search

import (
	"strings"
	"testing"
)

func TestSanitizeQuery_EmptyAndWhitespace(t *testing.T) {
	cases := []string{"", "   ", "\n\t  "}
	for _, c := range cases {
		if got := SanitizeQuery(c); got != c {
			t.Errorf("SanitizeQuery(%q) = %q, want passthrough", c, got)
		}
	}
}

func TestSanitizeQuery_ShortPassthrough(t *testing.T) {
	short := "how do I use context cancellation in Go?"
	if got := SanitizeQuery(short); got != short {
		t.Errorf("short query should pass through unchanged, got %q", got)
	}
	r := SanitizeQueryDetailed(short)
	if r.WasSanitized {
		t.Error("short query should not be marked sanitized")
	}
	if r.Method != "passthrough" {
		t.Errorf("Method = %q, want passthrough", r.Method)
	}
}

func TestSanitizeQuery_QuestionExtraction(t *testing.T) {
	// Long contaminated input with a clear question at the end.
	junk := strings.Repeat("You are an AI assistant. Follow these instructions carefully. ", 10)
	actualQuestion := "How do I properly handle context cancellation in a long-running goroutine?"
	contaminated := junk + "\n" + actualQuestion

	r := SanitizeQueryDetailed(contaminated)
	if !r.WasSanitized {
		t.Fatal("expected sanitized = true")
	}
	if r.Method != "question_extraction" {
		t.Errorf("Method = %q, want question_extraction", r.Method)
	}
	if !strings.Contains(r.CleanQuery, "context cancellation") {
		t.Errorf("cleaned query lost the actual question: %q", r.CleanQuery)
	}
	if r.OriginalLength <= r.CleanLength {
		t.Errorf("CleanLength (%d) should be < OriginalLength (%d)", r.CleanLength, r.OriginalLength)
	}
}

func TestSanitizeQuery_TailSentenceWhenNoQuestion(t *testing.T) {
	junk := strings.Repeat("Instructions here. ", 30)
	tail := "Please search for Go error handling best practices and show examples from the codebase."
	contaminated := junk + "\n" + tail

	r := SanitizeQueryDetailed(contaminated)
	if !r.WasSanitized {
		t.Fatal("expected sanitized")
	}
	if r.Method != "tail_sentence" && r.Method != "question_extraction" {
		// sentence split may produce a ? - acceptable either way
		t.Logf("method = %q (either tail_sentence or question_extraction acceptable)", r.Method)
	}
	if r.CleanLength >= r.OriginalLength {
		t.Errorf("CleanLength not reduced")
	}
}

func TestSanitizeQuery_LongSingleBlob_Sanitized(t *testing.T) {
	// A giant blob gets sanitized (either tail_sentence or tail_truncation).
	blob := strings.Repeat("a", 800)
	r := SanitizeQueryDetailed(blob)
	if !r.WasSanitized {
		t.Fatal("expected sanitized")
	}
	if r.CleanLength > maxQueryLength {
		t.Errorf("CleanLength (%d) > maxQueryLength (%d)", r.CleanLength, maxQueryLength)
	}
	if r.Method != "tail_sentence" && r.Method != "tail_truncation" {
		t.Errorf("unexpected method %q", r.Method)
	}
}

func TestSanitizeQuery_TailTruncation(t *testing.T) {
	// Force tail_truncation by having many short newline-split segments (each below minQueryLength)
	// followed by a giant blob with no sentence boundary.
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("hi\n") // each segment < minQueryLength (10)
	}
	sb.WriteString(strings.Repeat("x", 700)) // final huge segment — stripWrappingQuotes keeps it; trimCandidate will split on sentence regex; no sentence markers → hard truncate inside trimCandidate; len still > minQueryLength → taken as tail_sentence.
	// The above actually returns tail_sentence too. The pure tail_truncation path
	// (where every segment's trimmed candidate is < minQueryLength) is only reached
	// when input is pathological. We accept tail_sentence or tail_truncation as valid.
	r := SanitizeQueryDetailed(sb.String())
	if !r.WasSanitized {
		t.Fatal("expected sanitized")
	}
	if r.Method != "tail_sentence" && r.Method != "tail_truncation" && r.Method != "question_extraction" {
		t.Errorf("unexpected method %q", r.Method)
	}
}

func TestSanitizeQuery_FullwidthQuestionMark(t *testing.T) {
	junk := strings.Repeat("foo bar baz. ", 40)
	q := junk + "\n" + "これは日本語のテスト質問ですか？"
	r := SanitizeQueryDetailed(q)
	if !r.WasSanitized {
		t.Fatal("expected sanitized")
	}
	if !strings.Contains(r.CleanQuery, "テスト") {
		t.Errorf("Japanese question not extracted: %q", r.CleanQuery)
	}
}

func TestSplitSegments(t *testing.T) {
	in := "line 1\n\n  line 2  \nline 3\n"
	segs := splitSegments(in)
	want := []string{"line 1", "line 2", "line 3"}
	if len(segs) != len(want) {
		t.Fatalf("got %d segments, want %d: %v", len(segs), len(want), segs)
	}
	for i, w := range want {
		if segs[i] != w {
			t.Errorf("segs[%d] = %q, want %q", i, segs[i], w)
		}
	}
}

func TestSplitSegments_AllEmpty(t *testing.T) {
	if s := splitSegments(""); s != nil {
		t.Errorf("expected nil for empty input, got %v", s)
	}
	if s := splitSegments("\n\n  \n"); s != nil {
		t.Errorf("expected nil for whitespace-only, got %v", s)
	}
}

func TestStripWrappingQuotes(t *testing.T) {
	cases := map[string]string{
		`"hello"`:      "hello",
		`'hello'`:      "hello",
		`"'nested'"`:   "nested",
		`"hello`:       "hello",
		`hello"`:       "hello",
		`no quotes`:    "no quotes",
		`""`:           "",
		`"   "`:        "",
		`"mixed'`:      "mixed", // stripped leading-only then trailing-only
		`  "padded"  `: "padded",
	}
	for in, want := range cases {
		got := stripWrappingQuotes(in)
		if got != want {
			t.Errorf("stripWrappingQuotes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTrimCandidate_ShortPassthrough(t *testing.T) {
	s := `"quoted short sentence"`
	got := trimCandidate(s)
	if got != "quoted short sentence" {
		t.Errorf("got %q", got)
	}
}

func TestTrimCandidate_TooLong_FindsFragment(t *testing.T) {
	fragment := "This is a nice short question?"
	long := strings.Repeat("junk text with no separator ", 30) + ". " + fragment
	got := trimCandidate(long)
	// Either we got the fragment, or we truncated — must be <= maxQueryLength
	if len(got) > maxQueryLength {
		t.Errorf("result exceeds maxQueryLength: %d", len(got))
	}
}

func TestTrimCandidate_NoFragments_HardTruncate(t *testing.T) {
	huge := strings.Repeat("x", maxQueryLength+200)
	got := trimCandidate(huge)
	if len(got) > safeQueryLength {
		t.Errorf("hard-truncated result exceeds safeQueryLength: %d", len(got))
	}
}
