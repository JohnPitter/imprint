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
func (s *SessionService) buildContext(project string) []types.ContextBlock {
	var blocks []types.ContextBlock

	// Recent session summaries for this project.
	summaries, err := s.c.Summaries.ListByProject(project, 5)
	if err == nil && len(summaries) > 0 {
		var sb strings.Builder
		for _, sum := range summaries {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", sum.CreatedAt, sum.Title, sum.Narrative)
		}
		blocks = append(blocks, types.ContextBlock{
			Type:     "session-history",
			Label:    "Recent Sessions",
			Content:  sb.String(),
			Priority: 1,
		})
	}

	// High-importance compressed observations.
	obs, err := s.c.Observations.ListCompressedByImportance(project, 7, 10)
	if err == nil && len(obs) > 0 {
		var sb strings.Builder
		for _, o := range obs {
			fmt.Fprintf(&sb, "- [%s] %s", o.Type, o.Title)
			if o.Narrative != nil {
				sb.WriteString(": " + *o.Narrative)
			}
			sb.WriteString("\n")
		}
		blocks = append(blocks, types.ContextBlock{
			Type:     "key-observations",
			Label:    "Key Observations",
			Content:  sb.String(),
			Priority: 2,
		})
	}

	// High-strength memories.
	memories, err := s.c.Memories.ListByStrength(7, 10)
	if err == nil && len(memories) > 0 {
		var sb strings.Builder
		for _, m := range memories {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", m.Type, m.Title, m.Content)
		}
		blocks = append(blocks, types.ContextBlock{
			Type:     "memories",
			Label:    "Relevant Memories",
			Content:  sb.String(),
			Priority: 3,
		})
	}

	return blocks
}
