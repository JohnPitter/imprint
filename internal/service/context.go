package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"imprint/internal/types"
)

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
	c           *Container
	tokenBudget int
	layerBudget LayerBudget
	dataDir     string
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

	// ── L1 — Essential Story ─────────────────────────────────────────────
	l1Budget := s.layerBudget.L1EssentialStory
	var l1sb strings.Builder

	// 1a. Highest-strength memories (strength >= 7)
	memories, err := s.c.Memories.ListByStrength(7, 15)
	if err == nil && len(memories) > 0 {
		for _, m := range memories {
			line := fmt.Sprintf("- [%s] %s: %s\n", m.Type, m.Title, m.Content)
			if estimateTokens(l1sb.String()+line) > l1Budget*2/3 {
				break
			}
			l1sb.WriteString(line)
		}
	}

	// 1b. Most recent session summary for this project
	summaries, err := s.c.Summaries.ListByProject(project, 1)
	if err == nil && len(summaries) > 0 {
		sum := summaries[0]
		line := fmt.Sprintf("- [Last Session] %s: %s\n", sum.Title, sum.Narrative)
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
	l2Budget := s.layerBudget.L2SessionContext
	var l2sb strings.Builder

	// High-importance compressed observations from recent sessions for this project
	obs, err := s.c.Observations.ListCompressedByImportance(project, 6, 20)
	if err == nil && len(obs) > 0 {
		for _, o := range obs {
			narrative := ""
			if o.Narrative != nil {
				narrative = *o.Narrative
			}
			line := fmt.Sprintf("- [%s] %s: %s\n", o.Type, o.Title, narrative)
			if estimateTokens(l2sb.String()+line) > l2Budget {
				break
			}
			l2sb.WriteString(line)
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
			line := fmt.Sprintf("- [%s] %s: %s\n", sum.CreatedAt, sum.Title, sum.Narrative)
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

	return blocks, nil
}

// estimateTokens approximates token count from text length (~3 chars per token).
func estimateTokens(text string) int {
	return len(text) / 3
}
