package service

import (
	"encoding/json"
	"fmt"
	"time"

	"imprint/internal/store"
	"imprint/internal/types"

	"github.com/google/uuid"
)

// RememberService handles long-term memory operations.
type RememberService struct {
	c *Container
}

// NewRememberService creates a new RememberService.
func NewRememberService(c *Container) *RememberService {
	return &RememberService{c: c}
}

// Remember stores a new long-term memory.
func (s *RememberService) Remember(memType types.MemoryType, title, content string, concepts, files []string, strength int) (*store.MemoryRow, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if strength < 1 {
		strength = 5
	}
	if strength > 10 {
		strength = 10
	}

	now := store.TimeToString(time.Now())
	id := "mem_" + uuid.New().String()[:8]

	row := &store.MemoryRow{
		ID:                   id,
		CreatedAt:            now,
		UpdatedAt:            now,
		Type:                 string(memType),
		Title:                title,
		Content:              content,
		Concepts:             marshalToRaw(concepts),
		Files:                marshalToRaw(files),
		SessionIDs:           json.RawMessage("[]"),
		Strength:             strength,
		Version:              1,
		Supersedes:           json.RawMessage("[]"),
		SourceObservationIDs: json.RawMessage("[]"),
		IsLatest:             1,
	}

	if err := s.c.Memories.Create(row); err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}

	s.c.LogAudit("memory.create", id, "memory", map[string]any{"type": string(memType), "title": title})
	return row, nil
}

// Forget marks a memory as no longer latest (soft delete).
func (s *RememberService) Forget(id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	return s.c.Memories.Delete(id)
}

// EvolveInput collects the fields a user can edit on an existing memory.
// Empty/zero fields fall back to the previous version's value, so callers
// can send partial updates.
type EvolveInput struct {
	Content  string
	Title    string
	Type     string
	Strength int
}

// Evolve creates a new version of a memory via Supersede. Old version stays
// in place (is_latest=0) so the audit trail is preserved.
func (s *RememberService) Evolve(id string, in EvolveInput) (*store.MemoryRow, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	old, err := s.c.Memories.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get memory to evolve: %w", err)
	}

	content := in.Content
	if content == "" {
		content = old.Content
	}
	title := in.Title
	if title == "" {
		title = old.Title
	}
	memType := in.Type
	if memType == "" {
		memType = old.Type
	}
	strength := in.Strength
	if strength < 1 {
		strength = old.Strength
	}
	if strength > 10 {
		strength = 10
	}

	if content == old.Content && title == old.Title && memType == old.Type && strength == old.Strength {
		return old, nil // nothing changed; don't bump the version pointlessly
	}

	now := store.TimeToString(time.Now())
	newID := "mem_" + uuid.New().String()[:8]

	newMem := &store.MemoryRow{
		ID:                   newID,
		CreatedAt:            now,
		UpdatedAt:            now,
		Type:                 memType,
		Title:                title,
		Content:              content,
		Concepts:             old.Concepts,
		Files:                old.Files,
		SessionIDs:           old.SessionIDs,
		Strength:             strength,
		Version:              old.Version + 1,
		ParentID:             &id,
		SourceObservationIDs: old.SourceObservationIDs,
		IsLatest:             1,
	}

	if err := s.c.Memories.Supersede(id, newMem); err != nil {
		return nil, fmt.Errorf("supersede memory: %w", err)
	}

	s.c.LogAudit("memory.evolve", newID, "memory", map[string]any{
		"from": id, "title": title, "type": memType, "strength": strength,
	})

	return newMem, nil
}

func marshalToRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("[]")
	}
	return json.RawMessage(b)
}

// List returns memories with optional type filter and pagination.
func (s *RememberService) List(memType string, limit, offset int) ([]store.MemoryRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.c.Memories.List(memType, limit, offset)
}

// Count returns the total number of memories.
func (s *RememberService) Count() (int, error) {
	return s.c.Memories.Count()
}

// TopConcepts returns the most common concepts across latest memories.
func (s *RememberService) TopConcepts(limit int) ([]store.ConceptCount, error) {
	return s.c.Memories.TopConcepts(limit)
}
