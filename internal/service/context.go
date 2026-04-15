package service

import (
	"fmt"
	"strings"

	"imprint/internal/types"
)

// ContextService builds context blocks for injection, respecting a token budget.
type ContextService struct {
	c           *Container
	tokenBudget int
}

// NewContextService creates a new ContextService.
func NewContextService(c *Container, tokenBudget int) *ContextService {
	return &ContextService{c: c, tokenBudget: tokenBudget}
}

// BuildContext assembles context blocks from summaries, observations, and memories.
func (s *ContextService) BuildContext(sessionID, project string, budget int) ([]types.ContextBlock, error) {
	if budget <= 0 {
		budget = s.tokenBudget
	}

	var blocks []types.ContextBlock
	remaining := budget

	// 1. Recent session summaries
	summaries, err := s.c.Summaries.ListByProject(project, 5)
	if err == nil && len(summaries) > 0 {
		var sb strings.Builder
		for _, sum := range summaries {
			line := fmt.Sprintf("- [%s] %s: %s\n", sum.CreatedAt, sum.Title, sum.Narrative)
			if estimateTokens(sb.String()+line) > remaining/3 {
				break
			}
			sb.WriteString(line)
		}
		if sb.Len() > 0 {
			block := types.ContextBlock{
				Type:     "session-history",
				Label:    "Recent Sessions",
				Content:  sb.String(),
				Priority: 1,
			}
			blocks = append(blocks, block)
			remaining -= estimateTokens(block.Content)
		}
	}

	// 2. High-importance observations
	obs, err := s.c.Observations.ListCompressedByImportance(project, 7, 15)
	if err == nil && len(obs) > 0 {
		var sb strings.Builder
		for _, o := range obs {
			narrative := ""
			if o.Narrative != nil {
				narrative = *o.Narrative
			}
			line := fmt.Sprintf("- [%s] %s: %s\n", o.Type, o.Title, narrative)
			if estimateTokens(sb.String()+line) > remaining/3 {
				break
			}
			sb.WriteString(line)
		}
		if sb.Len() > 0 {
			block := types.ContextBlock{
				Type:     "key-observations",
				Label:    "Key Observations",
				Content:  sb.String(),
				Priority: 2,
			}
			blocks = append(blocks, block)
			remaining -= estimateTokens(block.Content)
		}
	}

	// 3. Strong memories
	memories, err := s.c.Memories.ListByStrength(6, 15)
	if err == nil && len(memories) > 0 {
		var sb strings.Builder
		for _, m := range memories {
			line := fmt.Sprintf("- [%s] %s: %s\n", m.Type, m.Title, m.Content)
			if estimateTokens(sb.String()+line) > remaining {
				break
			}
			sb.WriteString(line)
		}
		if sb.Len() > 0 {
			block := types.ContextBlock{
				Type:     "memories",
				Label:    "Relevant Memories",
				Content:  sb.String(),
				Priority: 3,
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// estimateTokens approximates token count from text length (~3 chars per token).
func estimateTokens(text string) int {
	return len(text) / 3
}
