package service

import (
	"encoding/json"
	"fmt"
	"time"

	"imprint/internal/store"

	"github.com/google/uuid"
)

// ActionService handles action, lease, and routine operations.
type ActionService struct {
	c *Container
}

// NewActionService creates a new ActionService.
func NewActionService(c *Container) *ActionService {
	return &ActionService{c: c}
}

// UpsertFromTask creates or updates an action from a Claude Code task completion.
// If an action with the same title exists, updates its status. Otherwise creates a new one.
func (s *ActionService) UpsertFromTask(title, description, status, sessionID string) (*store.ActionRow, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if status == "" {
		status = "done"
	}

	// Try to find existing action by title
	existing, err := s.c.Actions.List("", "", 200, 0)
	if err == nil {
		for _, a := range existing {
			if a.Title == title {
				a.Status = status
				a.Description = description
				a.UpdatedAt = store.TimeToString(time.Now())
				if err := s.c.Actions.Update(&a); err != nil {
					return nil, fmt.Errorf("update action: %w", err)
				}
				return &a, nil
			}
		}
	}

	// Create new action
	id := "act_" + uuid.New().String()[:8]
	now := store.TimeToString(time.Now())

	// Get project from session if available
	var project *string
	if sessionID != "" {
		if sess, err := s.c.Sessions.GetByID(sessionID); err == nil {
			project = &sess.Project
		}
	}

	row := &store.ActionRow{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    7,
		Project:     project,
		Tags:        json.RawMessage("[]"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.c.Actions.Create(row); err != nil {
		return nil, fmt.Errorf("create action from task: %w", err)
	}

	return row, nil
}

// CreateAction creates a new action.
func (s *ActionService) CreateAction(title, description, status string, priority int, project *string, tags []string) (*store.ActionRow, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if status == "" {
		status = "pending"
	}
	if priority == 0 {
		priority = 5
	}

	id := "act_" + uuid.New().String()[:8]

	var tagsRaw json.RawMessage
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsRaw = json.RawMessage(b)
	} else {
		tagsRaw = json.RawMessage("[]")
	}

	row := &store.ActionRow{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		Project:     project,
		Tags:        tagsRaw,
	}

	if err := s.c.Actions.Create(row); err != nil {
		return nil, fmt.Errorf("create action: %w", err)
	}

	return row, nil
}

// GetAction retrieves an action by ID.
func (s *ActionService) GetAction(id string) (*store.ActionRow, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.c.Actions.GetByID(id)
}

// ListActions returns actions with optional filters and pagination.
func (s *ActionService) ListActions(status, project string, limit, offset int) ([]store.ActionRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Actions.List(status, project, limit, offset)
}

// UpdateAction updates specific fields of an action.
func (s *ActionService) UpdateAction(id string, updates map[string]any) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	row, err := s.c.Actions.GetByID(id)
	if err != nil {
		return fmt.Errorf("get action for update: %w", err)
	}

	if v, ok := updates["title"]; ok {
		if str, ok := v.(string); ok {
			row.Title = str
		}
	}
	if v, ok := updates["description"]; ok {
		if str, ok := v.(string); ok {
			row.Description = str
		}
	}
	if v, ok := updates["status"]; ok {
		if str, ok := v.(string); ok {
			row.Status = str
		}
	}
	if v, ok := updates["priority"]; ok {
		switch p := v.(type) {
		case float64:
			row.Priority = int(p)
		case int:
			row.Priority = p
		}
	}
	if v, ok := updates["assignee"]; ok {
		if str, ok := v.(string); ok {
			row.Assignee = &str
		}
	}
	if v, ok := updates["project"]; ok {
		if str, ok := v.(string); ok {
			row.Project = &str
		}
	}
	if v, ok := updates["tags"]; ok {
		b, _ := json.Marshal(v)
		row.Tags = json.RawMessage(b)
	}
	if v, ok := updates["parentId"]; ok {
		if str, ok := v.(string); ok {
			row.ParentID = &str
		}
	}
	if v, ok := updates["sketchId"]; ok {
		if str, ok := v.(string); ok {
			row.SketchID = &str
		}
	}
	if v, ok := updates["crystallized"]; ok {
		switch c := v.(type) {
		case float64:
			row.Crystallized = int(c)
		case int:
			row.Crystallized = c
		case bool:
			if c {
				row.Crystallized = 1
			} else {
				row.Crystallized = 0
			}
		}
	}

	return s.c.Actions.Update(row)
}

// CreateEdge creates a dependency edge between two actions.
func (s *ActionService) CreateEdge(sourceID, targetID, edgeType string) (*store.ActionEdgeRow, error) {
	if sourceID == "" || targetID == "" {
		return nil, fmt.Errorf("sourceId and targetId are required")
	}
	if edgeType == "" {
		edgeType = "blocks"
	}

	id := "ae_" + uuid.New().String()[:8]

	edge := &store.ActionEdgeRow{
		ID:       id,
		SourceID: sourceID,
		TargetID: targetID,
		Type:     edgeType,
	}

	if err := s.c.Actions.CreateEdge(edge); err != nil {
		return nil, fmt.Errorf("create edge: %w", err)
	}

	return edge, nil
}

// GetFrontier returns pending actions with no unsatisfied blocking edges.
func (s *ActionService) GetFrontier() ([]store.ActionRow, error) {
	return s.c.Actions.GetFrontier()
}

// GetNext returns the highest priority action from the frontier.
func (s *ActionService) GetNext() (*store.ActionRow, error) {
	return s.c.Actions.GetNext()
}

// AcquireLease acquires a lease on an action for an agent.
func (s *ActionService) AcquireLease(actionID, agentID string, ttlSeconds int) (*store.LeaseRow, error) {
	if actionID == "" {
		return nil, fmt.Errorf("actionId is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("agentId is required")
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}

	locked, err := s.c.Leases.IsLocked(actionID)
	if err != nil {
		return nil, fmt.Errorf("check lease lock: %w", err)
	}
	if locked {
		return nil, fmt.Errorf("action %s is already locked", actionID)
	}

	now := time.Now()
	id := "ls_" + uuid.New().String()[:8]

	lease := &store.LeaseRow{
		ID:         id,
		ActionID:   actionID,
		AgentID:    agentID,
		AcquiredAt: store.TimeToString(now),
		ExpiresAt:  store.TimeToString(now.Add(time.Duration(ttlSeconds) * time.Second)),
		Status:     "active",
	}

	if err := s.c.Leases.Acquire(lease); err != nil {
		return nil, fmt.Errorf("acquire lease: %w", err)
	}

	return lease, nil
}

// ReleaseLease releases a lease and optionally records a result.
func (s *ActionService) ReleaseLease(leaseID, agentID string, result *string) error {
	if leaseID == "" {
		return fmt.Errorf("leaseId is required")
	}
	if agentID == "" {
		return fmt.Errorf("agentId is required")
	}
	return s.c.Leases.Release(leaseID, agentID, result)
}

// RenewLease extends a lease's expiration.
func (s *ActionService) RenewLease(leaseID, agentID string, ttlSeconds int) error {
	if leaseID == "" {
		return fmt.Errorf("leaseId is required")
	}
	if agentID == "" {
		return fmt.Errorf("agentId is required")
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}

	newExpiry := store.TimeToString(time.Now().Add(time.Duration(ttlSeconds) * time.Second))
	return s.c.Leases.Renew(leaseID, agentID, newExpiry)
}

// CreateRoutine creates a new routine.
func (s *ActionService) CreateRoutine(name string, steps, tags json.RawMessage, frozen int) (*store.RoutineRow, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	id := "rt_" + uuid.New().String()[:8]

	if len(steps) == 0 {
		steps = json.RawMessage("[]")
	}
	if len(tags) == 0 {
		tags = json.RawMessage("[]")
	}

	row := &store.RoutineRow{
		ID:     id,
		Name:   name,
		Steps:  steps,
		Tags:   tags,
		Frozen: frozen,
	}

	if err := s.c.Routines.Create(row); err != nil {
		return nil, fmt.Errorf("create routine: %w", err)
	}

	return row, nil
}

// GetRoutine retrieves a routine by ID.
func (s *ActionService) GetRoutine(id string) (*store.RoutineRow, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.c.Routines.GetByID(id)
}

// ListRoutines returns routines with pagination.
func (s *ActionService) ListRoutines(limit, offset int) ([]store.RoutineRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Routines.List(limit, offset)
}

// RunRoutine creates actions from a routine's steps.
func (s *ActionService) RunRoutine(routineID string) ([]store.ActionRow, error) {
	if routineID == "" {
		return nil, fmt.Errorf("routineId is required")
	}

	routine, err := s.c.Routines.GetByID(routineID)
	if err != nil {
		return nil, fmt.Errorf("get routine: %w", err)
	}

	var steps []struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    int      `json:"priority"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal(routine.Steps, &steps); err != nil {
		return nil, fmt.Errorf("parse routine steps: %w", err)
	}

	var created []store.ActionRow
	for _, step := range steps {
		action, err := s.CreateAction(step.Title, step.Description, "pending", step.Priority, nil, step.Tags)
		if err != nil {
			return nil, fmt.Errorf("create action from routine step: %w", err)
		}
		created = append(created, *action)
	}

	return created, nil
}
