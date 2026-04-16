package service

import (
	"encoding/json"
	"fmt"
	"time"

	"imprint/internal/store"

	"github.com/google/uuid"
)

// AdvancedService handles signals, checkpoints, sentinels, sketches,
// crystals, lessons, insights, facets, audit, and governance operations.
type AdvancedService struct {
	c *Container
}

// NewAdvancedService creates a new AdvancedService.
func NewAdvancedService(c *Container) *AdvancedService {
	return &AdvancedService{c: c}
}

// ---------------------------------------------------------------------------
// Signals
// ---------------------------------------------------------------------------

// SendSignal creates and stores a new signal between agents.
func (s *AdvancedService) SendSignal(from, to, content, sigType string) (*store.SignalRow, error) {
	if from == "" || to == "" {
		return nil, fmt.Errorf("from and to are required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if sigType == "" {
		sigType = "info"
	}

	id := "sig_" + uuid.New().String()[:8]

	row := &store.SignalRow{
		ID:        id,
		FromAgent: from,
		ToAgent:   to,
		Content:   content,
		Type:      sigType,
	}

	if err := s.c.Signals.Send(row); err != nil {
		return nil, fmt.Errorf("send signal: %w", err)
	}
	return row, nil
}

// ListSignals returns signals addressed to the given agent.
func (s *AdvancedService) ListSignals(agentID string, limit int) ([]store.SignalRow, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agentId is required")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.c.Signals.List(agentID, limit)
}

// ---------------------------------------------------------------------------
// Checkpoints
// ---------------------------------------------------------------------------

// CreateCheckpoint creates a new checkpoint.
func (s *AdvancedService) CreateCheckpoint(name, description, cpType string, actionID *string) (*store.CheckpointRow, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if cpType == "" {
		cpType = "approval"
	}

	id := "cp_" + uuid.New().String()[:8]

	row := &store.CheckpointRow{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        cpType,
		ActionID:    actionID,
	}

	if err := s.c.Checkpoints.Create(row); err != nil {
		return nil, fmt.Errorf("create checkpoint: %w", err)
	}
	return row, nil
}

// ResolveCheckpoint marks a checkpoint as resolved.
func (s *AdvancedService) ResolveCheckpoint(id, resolvedBy, result, status string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if resolvedBy == "" {
		return fmt.Errorf("resolvedBy is required")
	}
	if status == "" {
		status = "approved"
	}
	return s.c.Checkpoints.Resolve(id, resolvedBy, result, status)
}

// ListCheckpoints returns checkpoints with optional status filter.
func (s *AdvancedService) ListCheckpoints(status string, limit int) ([]store.CheckpointRow, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.c.Checkpoints.List(status, limit)
}

// ---------------------------------------------------------------------------
// Sentinels
// ---------------------------------------------------------------------------

// CreateSentinel creates a new sentinel watcher.
func (s *AdvancedService) CreateSentinel(name, sentinelType string, config map[string]any) (*store.SentinelRow, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if sentinelType == "" {
		sentinelType = "file_change"
	}

	id := "snt_" + uuid.New().String()[:8]

	var configRaw json.RawMessage
	if config != nil {
		b, _ := json.Marshal(config)
		configRaw = json.RawMessage(b)
	} else {
		configRaw = json.RawMessage("{}")
	}

	row := &store.SentinelRow{
		ID:     id,
		Name:   name,
		Type:   sentinelType,
		Config: configRaw,
	}

	if err := s.c.Sentinels.Create(row); err != nil {
		return nil, fmt.Errorf("create sentinel: %w", err)
	}
	return row, nil
}

// ListSentinels returns sentinels with optional status filter.
func (s *AdvancedService) ListSentinels(status string, limit int) ([]store.SentinelRow, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.c.Sentinels.List(status, limit)
}

// TriggerSentinel marks a sentinel as triggered with a result.
func (s *AdvancedService) TriggerSentinel(id, result string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Sentinels.Trigger(id, result)
}

// CancelSentinel marks a sentinel as cancelled.
func (s *AdvancedService) CancelSentinel(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Sentinels.Cancel(id)
}

// CheckSentinel retrieves the current state of a sentinel.
func (s *AdvancedService) CheckSentinel(id string) (*store.SentinelRow, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.c.Sentinels.Check(id)
}

// ---------------------------------------------------------------------------
// Sketches
// ---------------------------------------------------------------------------

// CreateSketch creates a new sketch (draft workspace).
func (s *AdvancedService) CreateSketch(title, description string, project *string, expiresInHours int) (*store.SketchRow, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if expiresInHours <= 0 {
		expiresInHours = 24
	}

	id := "sk_" + uuid.New().String()[:8]
	now := time.Now()

	row := &store.SketchRow{
		ID:          id,
		Title:       title,
		Description: description,
		Project:     project,
		CreatedAt:   store.TimeToString(now),
		ExpiresAt:   store.TimeToString(now.Add(time.Duration(expiresInHours) * time.Hour)),
	}

	if err := s.c.Sketches.Create(row); err != nil {
		return nil, fmt.Errorf("create sketch: %w", err)
	}
	return row, nil
}

// ListSketches returns sketches with optional status filter.
func (s *AdvancedService) ListSketches(status string, limit int) ([]store.SketchRow, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.c.Sketches.List(status, limit)
}

// AddToSketch appends an action ID to a sketch.
func (s *AdvancedService) AddToSketch(sketchID, actionID string) error {
	if sketchID == "" || actionID == "" {
		return fmt.Errorf("sketchId and actionId are required")
	}
	return s.c.Sketches.AddAction(sketchID, actionID)
}

// PromoteSketch marks a sketch as promoted.
func (s *AdvancedService) PromoteSketch(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Sketches.Promote(id)
}

// DiscardSketch marks a sketch as discarded.
func (s *AdvancedService) DiscardSketch(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Sketches.Discard(id)
}

// GarbageCollectSketches removes expired active sketches.
func (s *AdvancedService) GarbageCollectSketches() (int64, error) {
	return s.c.Sketches.GarbageCollect()
}

// ---------------------------------------------------------------------------
// Lessons
// ---------------------------------------------------------------------------

// CreateLesson creates a new lesson.
func (s *AdvancedService) CreateLesson(content, context, source string, project *string, tags []string) (*store.LessonRow, error) {
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if source == "" {
		source = "manual"
	}

	id := "les_" + uuid.New().String()[:8]

	var tagsRaw json.RawMessage
	if len(tags) > 0 {
		b, _ := json.Marshal(tags)
		tagsRaw = json.RawMessage(b)
	} else {
		tagsRaw = json.RawMessage("[]")
	}

	row := &store.LessonRow{
		ID:      id,
		Content: content,
		Context: context,
		Source:  source,
		Project: project,
		Tags:    tagsRaw,
	}

	if err := s.c.Lessons.Create(row); err != nil {
		return nil, fmt.Errorf("create lesson: %w", err)
	}
	return row, nil
}

// ListLessons returns lessons with optional project filter.
func (s *AdvancedService) ListLessons(project string, limit, offset int) ([]store.LessonRow, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.c.Lessons.List(project, limit, offset)
}

// SearchLessons performs a text search on lesson content.
func (s *AdvancedService) SearchLessons(query string, limit int) ([]store.LessonRow, error) {
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.c.Lessons.Search(query, limit)
}

// StrengthenLesson reinforces a lesson, boosting its confidence.
func (s *AdvancedService) StrengthenLesson(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Lessons.Strengthen(id)
}

// ---------------------------------------------------------------------------
// Insights
// ---------------------------------------------------------------------------

// ListInsights returns insights with optional project filter.
func (s *AdvancedService) ListInsights(project string, limit, offset int) ([]store.InsightRow, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.c.Insights.List(project, limit, offset)
}

// SearchInsights performs a text search on insight content and title.
func (s *AdvancedService) SearchInsights(query string, limit int) ([]store.InsightRow, error) {
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.c.Insights.Search(query, limit)
}

// ---------------------------------------------------------------------------
// Facets
// ---------------------------------------------------------------------------

// CreateFacet creates a new facet (tag/dimension on a target entity).
func (s *AdvancedService) CreateFacet(targetID, targetType, dimension, value string) (*store.FacetRow, error) {
	if targetID == "" || targetType == "" {
		return nil, fmt.Errorf("targetId and targetType are required")
	}
	if dimension == "" || value == "" {
		return nil, fmt.Errorf("dimension and value are required")
	}

	id := "fct_" + uuid.New().String()[:8]

	row := &store.FacetRow{
		ID:         id,
		TargetID:   targetID,
		TargetType: targetType,
		Dimension:  dimension,
		Value:      value,
	}

	if err := s.c.Facets.Create(row); err != nil {
		return nil, fmt.Errorf("create facet: %w", err)
	}
	return row, nil
}

// GetFacets returns all facets for a given target entity.
func (s *AdvancedService) GetFacets(targetID, targetType string) ([]store.FacetRow, error) {
	if targetID == "" || targetType == "" {
		return nil, fmt.Errorf("targetId and targetType are required")
	}
	return s.c.Facets.GetByTarget(targetID, targetType)
}

// RemoveFacet deletes a facet by ID.
func (s *AdvancedService) RemoveFacet(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Facets.Remove(id)
}

// QueryFacets returns facets matching a specific dimension and value.
func (s *AdvancedService) QueryFacets(dimension, value string, limit int) ([]store.FacetRow, error) {
	if dimension == "" || value == "" {
		return nil, fmt.Errorf("dimension and value are required")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.c.Facets.QueryByDimension(dimension, value, limit)
}

// FacetStats returns counts of facets grouped by dimension.
func (s *AdvancedService) FacetStats() (map[string]int, error) {
	return s.c.Facets.Stats()
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

// ListAudit returns audit log entries with optional action filter and pagination.
func (s *AdvancedService) ListAudit(action string, limit, offset int) ([]store.AuditRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Audit.List(action, limit, offset)
}

// ---------------------------------------------------------------------------
// Governance
// ---------------------------------------------------------------------------

// GovernanceDeleteMemory permanently deletes a memory and logs the action.
func (s *AdvancedService) GovernanceDeleteMemory(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	if err := s.c.Memories.HardDelete(id); err != nil {
		return fmt.Errorf("governance delete memory: %w", err)
	}

	auditID := "aud_" + uuid.New().String()[:8]
	auditRow := &store.AuditRow{
		ID:         auditID,
		Action:     "governance.delete",
		EntityID:   id,
		EntityType: "memory",
	}
	if err := s.c.Audit.Create(auditRow); err != nil {
		return fmt.Errorf("audit governance delete: %w", err)
	}

	return nil
}

// GovernanceBulkDelete permanently deletes multiple memories and logs each deletion.
func (s *AdvancedService) GovernanceBulkDelete(ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids list is empty")
	}

	deleted := 0
	for _, id := range ids {
		if err := s.c.Memories.HardDelete(id); err != nil {
			continue
		}

		auditID := "aud_" + uuid.New().String()[:8]
		auditRow := &store.AuditRow{
			ID:         auditID,
			Action:     "governance.bulk_delete",
			EntityID:   id,
			EntityType: "memory",
		}
		_ = s.c.Audit.Create(auditRow)
		deleted++
	}

	return deleted, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func marshalStringSlice(s []string) json.RawMessage {
	if len(s) == 0 {
		return json.RawMessage("[]")
	}
	b, _ := json.Marshal(s)
	return json.RawMessage(b)
}
