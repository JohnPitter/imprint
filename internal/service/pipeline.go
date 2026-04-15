package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"imprint/internal/pipeline"
	"imprint/internal/store"

	"github.com/google/uuid"
)

// PipelineService orchestrates summarization, consolidation, and lesson extraction.
type PipelineService struct {
	c            *Container
	summarizer   *pipeline.Summarizer
	consolidator *pipeline.Consolidator
	patterns     *pipeline.PatternDetector
}

// NewPipelineService creates a new PipelineService.
func NewPipelineService(c *Container, summarizer *pipeline.Summarizer, consolidator *pipeline.Consolidator) *PipelineService {
	return &PipelineService{
		c:            c,
		summarizer:   summarizer,
		consolidator: consolidator,
		patterns:     pipeline.NewPatternDetector(),
	}
}

// Summarize generates a session summary from compressed observations.
func (s *PipelineService) Summarize(ctx context.Context, sessionID string) (*store.SummaryRow, error) {
	// Get session to find project
	session, err := s.c.Sessions.GetByID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Get compressed observations for this session
	obs, err := s.c.Observations.ListCompressed(sessionID, 200, 0)
	if err != nil {
		return nil, fmt.Errorf("list observations: %w", err)
	}

	if len(obs) == 0 {
		return nil, fmt.Errorf("no compressed observations to summarize")
	}

	// Call LLM to summarize
	summary, err := s.summarizer.Summarize(ctx, sessionID, session.Project, obs)
	if err != nil {
		return nil, fmt.Errorf("summarize: %w", err)
	}

	// Store summary
	if err := s.c.Summaries.Create(summary); err != nil {
		return nil, fmt.Errorf("store summary: %w", err)
	}

	s.c.LogAudit("session.summarize", sessionID, "session", map[string]any{"title": summary.Title})
	log.Printf("[pipeline] Session %s summarized: %s", sessionID[:12], summary.Title)
	return summary, nil
}

// Consolidate runs the full consolidation pipeline for recent sessions.
// It groups compressed observations by concepts and produces long-term memories.
func (s *PipelineService) Consolidate(ctx context.Context, sessionID string) (int, error) {
	// Get session project
	session, err := s.c.Sessions.GetByID(sessionID)
	if err != nil {
		return 0, fmt.Errorf("get session: %w", err)
	}

	// Get high-importance compressed observations across recent sessions for this project
	obs, err := s.c.Observations.ListCompressedByImportance(session.Project, 5, 100)
	if err != nil {
		return 0, fmt.Errorf("list observations: %w", err)
	}

	if len(obs) < 3 {
		return 0, nil // not enough data to consolidate
	}

	// Run LLM consolidation
	memories, err := s.consolidator.Consolidate(ctx, obs)
	if err != nil {
		return 0, fmt.Errorf("consolidate: %w", err)
	}

	// Store each consolidated memory
	created := 0
	for _, cm := range memories {
		id := "mem_" + uuid.New().String()[:8]
		now := store.TimeToString(time.Now())
		conceptsJSON, _ := json.Marshal(cm.Concepts)
		filesJSON, _ := json.Marshal(cm.Files)

		strength := cm.Strength
		if strength < 1 {
			strength = 5
		}
		if strength > 10 {
			strength = 10
		}

		row := &store.MemoryRow{
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
			Type:      cm.Type,
			Title:     cm.Title,
			Content:   cm.Content,
			Concepts:  json.RawMessage(conceptsJSON),
			Files:     json.RawMessage(filesJSON),
			Strength:  strength,
			Version:   1,
			IsLatest:  1,
		}
		if err := s.c.Memories.Create(row); err != nil {
			log.Printf("[pipeline] Failed to store memory %s: %v", cm.Title, err)
			continue
		}
		created++
	}

	s.c.LogAudit("memory.consolidate", sessionID, "session", map[string]any{"created": created, "observations": len(obs)})
	log.Printf("[pipeline] Consolidated %d memories from %d observations", created, len(obs))

	// Also detect patterns and store as lessons
	patterns := s.patterns.DetectPatterns(obs)
	for _, p := range patterns {
		if p.Confidence < 0.5 {
			continue
		}
		lessonID := "les_" + uuid.New().String()[:8]
		now := store.TimeToString(time.Now())
		tagsJSON, _ := json.Marshal(p.Concepts)

		lesson := &store.LessonRow{
			ID:         lessonID,
			Content:    p.Description,
			Context:    p.Type,
			Confidence: p.Confidence,
			Source:     "consolidation",
			Tags:       json.RawMessage(tagsJSON),
			CreatedAt:  now,
			UpdatedAt:  now,
			DecayRate:  0.01,
		}
		if err := s.c.Lessons.Create(lesson); err != nil {
			log.Printf("[pipeline] Failed to store lesson: %v", err)
			continue
		}
	}

	if len(patterns) > 0 {
		log.Printf("[pipeline] Detected %d patterns, stored as lessons", len(patterns))
	}

	return created, nil
}

// RunFullPipeline runs summarize + consolidate for a session end.
func (s *PipelineService) RunFullPipeline(ctx context.Context, sessionID string) error {
	// 1. Summarize
	if _, err := s.Summarize(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Summarize failed for %s: %v", sessionID[:min(12, len(sessionID))], err)
		// Continue to consolidation even if summarize fails
	}

	// 2. Consolidate
	if _, err := s.Consolidate(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Consolidate failed for %s: %v", sessionID[:min(12, len(sessionID))], err)
	}

	return nil
}
