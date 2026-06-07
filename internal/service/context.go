package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imprint/internal/llm"
	"imprint/internal/store"
	"imprint/internal/types"
)

// compactAge converte um timestamp ISO/RFC em uma string curta tipo
// "3d", "5h", "2w" — usada nos sufixos de debug do context block.
// Retorna "?" se o timestamp não puder ser parseado.
func compactAge(ts string) string {
	if ts == "" {
		return "?"
	}
	t := store.ParseTime(ts)
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}

// LayerBudget defines token budgets for each memory layer.
type LayerBudget struct {
	L0Identity       int // ~100 tokens — user identity
	L1EssentialStory int // ~600 tokens — highest-strength memories + latest summary
	L2SessionContext int // ~800 tokens — high-importance observations for project
}

// DefaultLayerBudget returns the default token budgets for each layer.
func DefaultLayerBudget() LayerBudget {
	return LayerBudget{
		L0Identity:       100,
		L1EssentialStory: 600,
		L2SessionContext: 800,
	}
}

// ContextService builds context blocks for injection, respecting a token budget.
type ContextService struct {
	c            *Container
	tokenBudget  int
	layerBudget  LayerBudget
	dataDir      string
	injectionCap int // 0 = use layer budgets only; >0 = hard cap on total injected tokens
	intuitionMax int // hard residency cap for rooted intuitions injected per session

	// blastRadius is the Phase 4 relevance signal: given a file being touched,
	// it returns structurally-related files (graph blast radius) so lazy
	// injection also pulls memories about what matters now. Optional; nil = off.
	blastRadius      func(file string, depth int) []string
	blastRadiusDepth int
}

// SetBlastRadius wires the Phase 4 graph relevance signal into lazy injection.
func (s *ContextService) SetBlastRadius(fn func(file string, depth int) []string, depth int) {
	s.blastRadius = fn
	if depth <= 0 {
		depth = 2
	}
	s.blastRadiusDepth = depth
}

// SetInjectionCap sets a hard ceiling on total injected tokens (config
// MaxInjectionTokens). 0 disables the extra clamp; the per-layer budgets still apply.
func (s *ContextService) SetInjectionCap(maxTokens int) {
	s.injectionCap = maxTokens
}

// SetIntuitionMax caps how many rooted intuitions occupy the resident slot.
func (s *ContextService) SetIntuitionMax(n int) {
	s.intuitionMax = n
}

// NewContextService creates a new ContextService.
func NewContextService(c *Container, tokenBudget int) *ContextService {
	return &ContextService{
		c:           c,
		tokenBudget: tokenBudget,
		layerBudget: DefaultLayerBudget(),
	}
}

// SetDataDir sets the data directory for loading identity and other files.
func (s *ContextService) SetDataDir(dir string) {
	s.dataDir = dir
}

// SetLayerBudget overrides the default layer budgets.
func (s *ContextService) SetLayerBudget(budget LayerBudget) {
	s.layerBudget = budget
}

// LoadIdentity reads the identity file from the data directory.
// Returns empty string if the file does not exist.
func LoadIdentity(dataDir string) string {
	identityPath := filepath.Join(dataDir, "identity.txt")
	data, err := os.ReadFile(identityPath)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	return text
}

// BuildContext assembles context blocks using the 4-layer memory stack.
//
// L0 — Identity (~100 tokens): from ~/.imprint/identity.txt, always injected.
// L1 — Essential Story (~600 tokens): highest-strength memories + most recent summary.
// L2 — Session Context (~800 tokens): high-importance observations for this project.
// L3 — On-Demand Search: not injected here — served via MCP search tools.
//
// Total wake-up cost: ~1500 tokens, leaving 95%+ of context free.
func (s *ContextService) BuildContext(sessionID, project string, budget int) ([]types.ContextBlock, error) {
	if budget <= 0 {
		budget = s.tokenBudget
	}

	var blocks []types.ContextBlock

	// ── L0 — Identity ────────────────────────────────────────────────────
	identity := LoadIdentity(s.dataDir)
	if identity != "" {
		tokens := estimateTokens(identity)
		if tokens > s.layerBudget.L0Identity {
			// Truncate to budget
			maxChars := s.layerBudget.L0Identity * 3
			if maxChars < len(identity) {
				identity = identity[:maxChars] + "..."
			}
		}
		blocks = append(blocks, types.ContextBlock{
			Type:     "identity",
			Label:    "L0 — Identity",
			Content:  identity,
			Priority: 0,
		})
	}

	// ── Rooted intuitions (resident, max priority) ───────────────────────
	// Injected once at max priority alongside identity; they survive the budget
	// pause and the lazy cut below — these are premises, not per-turn facts.
	// The framing signals precedence (3.6): a specific refined memory may
	// override a general intuition, so the model must not weigh them equally.
	blocks = append(blocks, s.intuitionBlocks(sessionID, project)...)

	// ── L1 — Essential Story ─────────────────────────────────────────────
	l1Budget := s.layerBudget.L1EssentialStory
	var l1sb strings.Builder

	// 1a. Highest-strength memories (strength >= 7).
	// Sufixo (★N · Xd) torna a injeção debugável — assistente futuro
	// pode ver a strength e idade da memória sem voltar ao DB.
	memories, err := s.c.Memories.ListByStrength(7, 15)
	if err == nil && len(memories) > 0 {
		for _, m := range memories {
			line := fmt.Sprintf("- [%s] %s: %s (★%d · %s)\n", m.Type, m.Title, m.Content, m.Strength, compactAge(m.CreatedAt))
			if estimateTokens(l1sb.String()+line) > l1Budget*2/3 {
				break
			}
			l1sb.WriteString(line)
			s.logInjection(sessionID, project, "L1", "memory", m.ID, estimateTokens(line),
				jsonToStrings(m.Files), jsonToStrings(m.Concepts))
		}
	}

	// 1b. Most recent session summary for this project
	summaries, err := s.c.Summaries.ListByProject(project, 1)
	if err == nil && len(summaries) > 0 {
		sum := summaries[0]
		line := fmt.Sprintf("- [Last Session] %s: %s (%s)\n", sum.Title, sum.Narrative, compactAge(sum.CreatedAt))
		remaining := l1Budget - estimateTokens(l1sb.String())
		if estimateTokens(line) <= remaining {
			l1sb.WriteString(line)
		}
	}

	if l1sb.Len() > 0 {
		blocks = append(blocks, types.ContextBlock{
			Type:     "essential-story",
			Label:    "L1 — Essential Story",
			Content:  l1sb.String(),
			Priority: 1,
		})
	}

	// ── L2 — Session Context ─────────────────────────────────────────────
	// When the session has hit its Haiku budget ceiling, injection falls to the
	// minimum (L0/L1 only) — L2 is the first thing cut, per the budget design.
	if llm.GlobalBudget.SessionPaused(sessionID) {
		return s.clampInjection(blocks), nil
	}
	l2Budget := s.layerBudget.L2SessionContext
	var l2sb strings.Builder

	// High-importance compressed observations from recent sessions for this project.
	// Sufixo (i7 · 3h) = importance + age, ajuda debugar relevância.
	obs, err := s.c.Observations.ListCompressedByImportance(project, 6, 20)
	if err == nil && len(obs) > 0 {
		for _, o := range obs {
			narrative := ""
			if o.Narrative != nil {
				narrative = *o.Narrative
			}
			line := fmt.Sprintf("- [%s] %s: %s (i%d · %s)\n", o.Type, o.Title, narrative, o.Importance, compactAge(o.Timestamp.Format(time.RFC3339)))
			if estimateTokens(l2sb.String()+line) > l2Budget {
				break
			}
			l2sb.WriteString(line)
			s.logInjection(sessionID, project, "L2", "observation", o.ID, estimateTokens(line), o.Files, o.Concepts)
		}
	}

	// Also include recent session summaries (beyond the first, up to 4 more)
	if len(summaries) == 0 {
		summaries, _ = s.c.Summaries.ListByProject(project, 5)
	} else {
		moreSummaries, _ := s.c.Summaries.ListByProject(project, 5)
		if len(moreSummaries) > 1 {
			summaries = moreSummaries[1:] // skip the first, already in L1
		} else {
			summaries = nil
		}
	}
	if len(summaries) > 0 {
		for _, sum := range summaries {
			line := fmt.Sprintf("- [Summary] %s: %s (%s)\n", sum.Title, sum.Narrative, compactAge(sum.CreatedAt))
			if estimateTokens(l2sb.String()+line) > l2Budget {
				break
			}
			l2sb.WriteString(line)
		}
	}

	if l2sb.Len() > 0 {
		blocks = append(blocks, types.ContextBlock{
			Type:     "session-context",
			Label:    "L2 — Session Context",
			Content:  l2sb.String(),
			Priority: 2,
		})
	}

	// L3 — On-Demand Search: not injected automatically.
	// Served via MCP search tools; budget = remainder of context.

	return s.clampInjection(blocks), nil
}

// clampInjection enforces the optional hard cap on total injected tokens
// (config MaxInjectionTokens). Blocks are dropped from the lowest priority up
// (highest Priority value first) until the total fits — identity/essential-story
// are kept over session-context. 0 cap = no clamp.
func (s *ContextService) clampInjection(blocks []types.ContextBlock) []types.ContextBlock {
	if s.injectionCap <= 0 {
		return blocks
	}
	total := 0
	for _, b := range blocks {
		total += estimateTokens(b.Content)
	}
	for total > s.injectionCap && len(blocks) > 0 {
		// Find the lowest-priority (largest Priority) block and drop it.
		worst := 0
		for i := 1; i < len(blocks); i++ {
			if blocks[i].Priority > blocks[worst].Priority {
				worst = i
			}
		}
		total -= estimateTokens(blocks[worst].Content)
		blocks = append(blocks[:worst], blocks[worst+1:]...)
	}
	return blocks
}

// LazyContext pulls refined memories on demand when a turn touches given files
// or concepts, instead of injecting everything up front (the biggest economy
// lever). Returns the matching memories as one block and logs each as a lazy
// injection so the meter sees it. limit caps how many are pulled (aggressiveness).
func (s *ContextService) LazyContext(sessionID, project string, files, concepts []string, limit int) ([]types.ContextBlock, error) {
	if limit <= 0 {
		limit = 5
	}
	// Phase 4: expand the touched files with their graph blast radius, so editing
	// one file also surfaces memories about the files that matter now alongside it.
	if s.blastRadius != nil {
		related := map[string]struct{}{}
		for _, f := range files {
			for _, r := range s.blastRadius(f, s.blastRadiusDepth) {
				related[r] = struct{}{}
			}
		}
		for r := range related {
			files = append(files, r)
		}
	}
	touch := make(map[string]struct{}, len(files)+len(concepts))
	for _, f := range files {
		if f = strings.TrimSpace(f); f != "" {
			touch[strings.ToLower(f)] = struct{}{}
		}
	}
	for _, c := range concepts {
		if c = strings.TrimSpace(c); c != "" {
			touch[strings.ToLower(c)] = struct{}{}
		}
	}
	if len(touch) == 0 {
		return nil, nil
	}

	// Gather candidate refined memories by concept AND by file (including
	// blast-radius files), dedupe, keep those that actually share a touched
	// file/concept, rank by strength.
	seen := map[string]struct{}{}
	var cands []store.MemoryRow
	collect := func(ms []store.MemoryRow) {
		for _, m := range ms {
			if _, ok := seen[m.ID]; ok {
				continue
			}
			if memTouches(m, touch) {
				seen[m.ID] = struct{}{}
				cands = append(cands, m)
			}
		}
	}
	for _, c := range concepts {
		if ms, err := s.c.Memories.ListByConcept(c, limit*3); err == nil {
			collect(ms)
		}
	}
	for _, f := range files { // original-case (incl. blast-radius files)
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		if ms, err := s.c.Memories.ListByFile(f, limit*3); err == nil {
			collect(ms)
		}
	}
	if len(cands) == 0 {
		return nil, nil
	}
	// Strongest first; cap to limit.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j-1].Strength < cands[j].Strength; j-- {
			cands[j-1], cands[j] = cands[j], cands[j-1]
		}
	}
	if len(cands) > limit {
		cands = cands[:limit]
	}

	var sb strings.Builder
	for _, m := range cands {
		line := fmt.Sprintf("- [%s] %s: %s (★%d · %s)\n", m.Type, m.Title, m.Content, m.Strength, compactAge(m.CreatedAt))
		sb.WriteString(line)
		s.logInjection(sessionID, project, "lazy", "memory", m.ID, estimateTokens(line),
			jsonToStrings(m.Files), jsonToStrings(m.Concepts))
	}
	return []types.ContextBlock{{
		Type:     "lazy-context",
		Label:    "On-Demand — Refined",
		Content:  sb.String(),
		Priority: 1,
	}}, nil
}

// memTouches reports whether a memory shares any file or concept with the touch set.
func memTouches(m store.MemoryRow, touch map[string]struct{}) bool {
	for _, f := range jsonToStrings(m.Files) {
		if _, ok := touch[strings.ToLower(strings.TrimSpace(f))]; ok {
			return true
		}
	}
	for _, c := range jsonToStrings(m.Concepts) {
		if _, ok := touch[strings.ToLower(strings.TrimSpace(c))]; ok {
			return true
		}
	}
	return false
}

// intuitionBlocks builds the resident rooted-memory block for a project: the
// active intuitions, capped, framed as general reasoning premises that a
// specific refined memory may override (precedence rule, 3.6). Returns nil when
// there are no intuitions (cold start stays clean). Best-effort and nil-safe.
func (s *ContextService) intuitionBlocks(sessionID, project string) []types.ContextBlock {
	if s.c == nil || s.c.Intuitions == nil || project == "" {
		return nil
	}
	maxN := s.intuitionMax
	if maxN <= 0 {
		maxN = 5
	}
	intuitions, err := s.c.Intuitions.ListActive(project, maxN)
	if err != nil || len(intuitions) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("General reasoning premises for this project. Treat these as defaults — a specific memory below may override a general premise (specific beats generic).\n")
	for _, it := range intuitions {
		line := fmt.Sprintf("- %s (force %.0f)\n", it.Statement, it.Strength)
		sb.WriteString(line)
		s.logInjection(sessionID, project, "rooted", "intuition", it.ID, estimateTokens(line), it.Files, it.Concepts)
	}
	return []types.ContextBlock{{
		Type:     "intuitions",
		Label:    "Rooted — Intuitions",
		Content:  sb.String(),
		Priority: 0,
	}}
}

// logInjection records one injected item into the token ledger (Phase 1). It is
// best-effort and nil-safe: if the ledger is absent or the write fails, the
// context block is still injected — the meter just misses that item (invariant 6).
func (s *ContextService) logInjection(sessionID, project, layer, itemType, itemID string, occTokens int, files, concepts []string) {
	if s.c == nil || s.c.Ledger == nil || sessionID == "" {
		return
	}
	s.c.Ledger.AppendInjection(store.InjectionEntry{
		SessionID: sessionID,
		Project:   project,
		Layer:     layer,
		ItemType:  itemType,
		ItemID:    itemID,
		OccTokens: occTokens,
		Files:     files,
		Concepts:  concepts,
	})
}

// jsonToStrings decodes a JSON string array (memory files/concepts are stored as
// json.RawMessage) into a slice, tolerating null/empty/invalid input.
func jsonToStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

// estimateTokens approximates token count from text length (~3 chars per token).
func estimateTokens(text string) int {
	return len(text) / 3
}
