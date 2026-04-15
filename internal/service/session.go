package service

import (
	"fmt"
	"strings"
	"time"

	"imprint/internal/store"
	"imprint/internal/types"

	"github.com/google/uuid"
)

// SessionService handles session lifecycle operations.
type SessionService struct {
	c *Container
}

// NewSessionService creates a new SessionService.
func NewSessionService(c *Container) *SessionService {
	return &SessionService{c: c}
}

// Start creates a new session and returns context blocks for injection.
func (s *SessionService) Start(sessionID, project, cwd string) (*store.SessionRow, []types.ContextBlock, error) {
	if sessionID == "" {
		sessionID = "ses_" + uuid.New().String()[:8]
	}
	if project == "" {
		return nil, nil, fmt.Errorf("project is required")
	}

	row := &store.SessionRow{
		ID:               sessionID,
		Project:          project,
		Cwd:              cwd,
		StartedAt:        time.Now(),
		Status:           types.SessionActive,
		ObservationCount: 0,
	}

	if err := s.c.Sessions.Create(row); err != nil {
		return nil, nil, fmt.Errorf("create session: %w", err)
	}

	s.c.LogAudit("session.start", sessionID, "session", map[string]any{"project": project})

	blocks := s.buildContext(project)
	return row, blocks, nil
}

// End marks a session as completed.
func (s *SessionService) End(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionId is required")
	}
	err := s.c.Sessions.End(sessionID)
	if err == nil {
		s.c.LogAudit("session.end", sessionID, "session", nil)
	}
	return err
}

// List returns sessions with pagination.
func (s *SessionService) List(project string, limit, offset int) ([]store.SessionRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Sessions.List(project, limit, offset)
}

// buildContext assembles context blocks from recent summaries and high-importance observations.
// Uses the same layered approach as ContextService but without needing a full service instance.
func (s *SessionService) buildContext(project string) []types.ContextBlock {
	var blocks []types.ContextBlock

	// L1 — Essential Story: high-strength memories + most recent summary
	var l1sb strings.Builder

	memories, err := s.c.Memories.ListByStrength(7, 10)
	if err == nil && len(memories) > 0 {
		for _, m := range memories {
			fmt.Fprintf(&l1sb, "- [%s] %s: %s\n", m.Type, m.Title, m.Content)
		}
	}

	summaries, err := s.c.Summaries.ListByProject(project, 5)
	if err == nil && len(summaries) > 0 {
		fmt.Fprintf(&l1sb, "- [Last Session] %s: %s\n", summaries[0].Title, summaries[0].Narrative)
	}

	if l1sb.Len() > 0 {
		blocks = append(blocks, types.ContextBlock{
			Type:     "essential-story",
			Label:    "L1 — Essential Story",
			Content:  l1sb.String(),
			Priority: 1,
		})
	}

	// L2 — Session Context: high-importance observations + older summaries
	var l2sb strings.Builder

	obs, err := s.c.Observations.ListCompressedByImportance(project, 6, 15)
	if err == nil && len(obs) > 0 {
		for _, o := range obs {
			fmt.Fprintf(&l2sb, "- [%s] %s", o.Type, o.Title)
			if o.Narrative != nil {
				l2sb.WriteString(": " + *o.Narrative)
			}
			l2sb.WriteString("\n")
		}
	}

	if len(summaries) > 1 {
		for _, sum := range summaries[1:] {
			fmt.Fprintf(&l2sb, "- [%s] %s: %s\n", sum.CreatedAt, sum.Title, sum.Narrative)
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

	return blocks
}
