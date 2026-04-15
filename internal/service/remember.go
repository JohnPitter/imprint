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

// Evolve updates a memory's content by creating a new version via Supersede.
func (s *RememberService) Evolve(id string, newContent string, newStrength int) (*store.MemoryRow, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if newContent == "" {
		return nil, fmt.Errorf("content is required")
	}

	old, err := s.c.Memories.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get memory to evolve: %w", err)
	}

	if newStrength < 1 {
		newStrength = old.Strength
	}
	if newStrength > 10 {
		newStrength = 10
	}

	now := store.TimeToString(time.Now())
	newID := "mem_" + uuid.New().String()[:8]

	newMem := &store.MemoryRow{
		ID:                   newID,
		CreatedAt:            now,
		UpdatedAt:            now,
		Type:                 old.Type,
		Title:                old.Title,
		Content:              newContent,
		Concepts:             old.Concepts,
		Files:                old.Files,
		SessionIDs:           old.SessionIDs,
		Strength:             newStrength,
		Version:              old.Version + 1,
		SourceObservationIDs: old.SourceObservationIDs,
		IsLatest:             1,
	}

	if err := s.c.Memories.Supersede(id, newMem); err != nil {
		return nil, fmt.Errorf("supersede memory: %w", err)
	}

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
