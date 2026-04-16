package service

import (
	"fmt"
	"time"

	"imprint/internal/store"
	"imprint/internal/types"

	"github.com/google/uuid"
)

// SessionService handles session lifecycle operations.
type SessionService struct {
	c          *Container
	contextSvc *ContextService
}

// NewSessionService creates a new SessionService.
func NewSessionService(c *Container, contextSvc *ContextService) *SessionService {
	return &SessionService{c: c, contextSvc: contextSvc}
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

	// Use the 4-layer ContextService instead of duplicated logic
	var blocks []types.ContextBlock
	if s.contextSvc != nil {
		blocks, _ = s.contextSvc.BuildContext(sessionID, project, 0)
	}
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
