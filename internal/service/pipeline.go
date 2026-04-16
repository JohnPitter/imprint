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

// PipelineService orchestrates summarization, consolidation, reflection, graph extraction, and lesson extraction.
type PipelineService struct {
	c            *Container
	summarizer   *pipeline.Summarizer
	consolidator *pipeline.Consolidator
	reflector    *pipeline.Reflector
	graphSvc     *GraphService
	patterns     *pipeline.PatternDetector
}

// NewPipelineService creates a new PipelineService.
func NewPipelineService(c *Container, summarizer *pipeline.Summarizer, consolidator *pipeline.Consolidator, reflector *pipeline.Reflector, graphSvc *GraphService) *PipelineService {
	return &PipelineService{
		c:            c,
		summarizer:   summarizer,
		consolidator: consolidator,
		reflector:    reflector,
		graphSvc:     graphSvc,
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

	// Also detect patterns and store as lessons (with dedup)
	patterns := s.patterns.DetectPatterns(obs)
	for _, p := range patterns {
		if p.Confidence < 0.5 {
			continue
		}

		// Dedup: check if a lesson with the same content already exists
		existing, _ := s.c.Lessons.Search(p.Description[:min(50, len(p.Description))], 1)
		if len(existing) > 0 {
			// Strengthen existing lesson instead of creating duplicate
			s.c.Lessons.Strengthen(existing[0].ID)
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

// Reflect generates insights from existing memories and observations via LLM.
func (s *PipelineService) Reflect(ctx context.Context, sessionID string) (int, error) {
	session, err := s.c.Sessions.GetByID(sessionID)
	if err != nil {
		return 0, fmt.Errorf("get session: %w", err)
	}

	// Get strong memories and recent observations
	memories, err := s.c.Memories.ListByStrength(5, 30)
	if err != nil {
		return 0, fmt.Errorf("list memories: %w", err)
	}

	obs, err := s.c.Observations.ListCompressedByImportance(session.Project, 5, 30)
	if err != nil {
		return 0, fmt.Errorf("list observations: %w", err)
	}

	if len(memories) < 3 && len(obs) < 3 {
		return 0, nil // not enough data
	}

	// Call LLM to reflect
	insights, err := s.reflector.Reflect(ctx, memories, obs)
	if err != nil {
		return 0, fmt.Errorf("reflect: %w", err)
	}

	// Store insights
	created := 0
	for _, ins := range insights {
		id := "ins_" + uuid.New().String()[:8]
		now := store.TimeToString(time.Now())
		conceptsJSON, _ := json.Marshal(ins.Concepts)

		confidence := ins.Confidence
		if confidence <= 0 || confidence > 1 {
			confidence = 0.5
		}

		row := &store.InsightRow{
			ID:                   id,
			Title:                ins.Title,
			Content:              ins.Content,
			Confidence:           confidence,
			SourceConceptCluster: json.RawMessage(conceptsJSON),
			CreatedAt:            now,
			UpdatedAt:            now,
			DecayRate:            0.01,
		}
		if err := s.c.Insights.Create(row); err != nil {
			log.Printf("[pipeline] Failed to store insight %s: %v", ins.Title, err)
			continue
		}
		created++
	}

	if created > 0 {
		s.c.LogAudit("insight.reflect", sessionID, "session", map[string]any{"created": created})
		log.Printf("[pipeline] Generated %d insights from reflection", created)
	}

	return created, nil
}

// ExtractGraph processes compressed observations to build the knowledge graph.
func (s *PipelineService) ExtractGraph(ctx context.Context, sessionID string) (int, error) {
	obs, err := s.c.Observations.ListCompressed(sessionID, 50, 0)
	if err != nil {
		return 0, fmt.Errorf("list observations: %w", err)
	}

	if len(obs) == 0 || s.graphSvc == nil {
		return 0, nil
	}

	// Process up to 10 observations (each makes an LLM call)
	processed := 0
	limit := min(len(obs), 10)
	for i := range limit {
		if err := s.graphSvc.ExtractAndStore(ctx, &obs[i]); err != nil {
			log.Printf("[pipeline] Graph extraction failed for obs %s: %v", obs[i].ID, err)
			continue
		}
		processed++
	}

	if processed > 0 {
		s.c.LogAudit("graph.extract", sessionID, "session", map[string]any{"processed": processed})
		log.Printf("[pipeline] Extracted graph entities from %d observations", processed)
	}

	return processed, nil
}

// ExtractActions creates action items from high-importance observations (heuristic, no LLM).
// Active sessions produce in_progress actions; completed sessions produce done actions.
func (s *PipelineService) ExtractActions(ctx context.Context, sessionID string) (int, error) {
	session, err := s.c.Sessions.GetByID(sessionID)
	if err != nil {
		return 0, fmt.Errorf("get session: %w", err)
	}

	obs, err := s.c.Observations.ListCompressed(sessionID, 100, 0)
	if err != nil {
		return 0, fmt.Errorf("list observations: %w", err)
	}

	status := "in_progress"
	if session.Status != "active" {
		status = "done"
	}

	created := 0
	for _, o := range obs {
		if o.Importance < 7 {
			continue
		}

		// Dedup by title
		if exists, _ := s.c.Actions.ExistsByTitle(o.Title); exists {
			continue
		}

		id := "act_" + uuid.New().String()[:8]
		now := store.TimeToString(time.Now())

		var project *string
		if session.Project != "" {
			project = &session.Project
		}

		tagsJSON, _ := json.Marshal(o.Concepts)
		row := &store.ActionRow{
			ID:          id,
			Title:       o.Title,
			Description: "",
			Status:      status,
			Priority:    o.Importance,
			Project:     project,
			Tags:        json.RawMessage(tagsJSON),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if o.Narrative != nil {
			row.Description = *o.Narrative
		}

		if err := s.c.Actions.Create(row); err != nil {
			continue
		}
		created++
	}

	if created > 0 {
		s.c.LogAudit("action.extract", sessionID, "session", map[string]any{"created": created, "status": status})
		log.Printf("[pipeline] Created %d actions (%s) from high-importance observations", created, status)
	}

	return created, nil
}

// RunFullPipeline runs all pipeline stages for a session end.
func (s *PipelineService) RunFullPipeline(ctx context.Context, sessionID string) error {
	sid := sessionID[:min(12, len(sessionID))]

	// 1. Summarize
	if _, err := s.Summarize(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Summarize failed for %s: %v", sid, err)
	}

	// 2. Consolidate (memories + lessons from patterns)
	if _, err := s.Consolidate(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Consolidate failed for %s: %v", sid, err)
	}

	// 3. Extract knowledge graph entities
	if _, err := s.ExtractGraph(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Graph extraction failed for %s: %v", sid, err)
	}

	// 4. Extract actions from high-importance observations
	if _, err := s.ExtractActions(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Action extraction failed for %s: %v", sid, err)
	}

	// 5. Reflect (generate insights from memories + observations)
	if _, err := s.Reflect(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Reflect failed for %s: %v", sid, err)
	}

	return nil
}

// RunFinalize runs only the final-pass stages that the periodic scheduler doesn't cover.
// Called by the session-end hook after the session is marked completed.
// The scheduler already handles summarize + consolidate during the session.
func (s *PipelineService) RunFinalize(ctx context.Context, sessionID string) error {
	sid := sessionID[:min(12, len(sessionID))]
	log.Printf("[pipeline] Finalizing session %s", sid)

	// One final summarize + consolidate to catch any last observations
	if _, err := s.Summarize(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Final summarize failed for %s: %v", sid, err)
	}
	if _, err := s.Consolidate(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Final consolidate failed for %s: %v", sid, err)
	}

	// Graph extraction
	if _, err := s.ExtractGraph(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Graph extraction failed for %s: %v", sid, err)
	}

	// Actions
	if _, err := s.ExtractActions(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Action extraction failed for %s: %v", sid, err)
	}

	// Reflect
	if _, err := s.Reflect(ctx, sessionID); err != nil {
		log.Printf("[pipeline] Reflect failed for %s: %v", sid, err)
	}

	// Mark all in_progress actions for this project as done
	session, err := s.c.Sessions.GetByID(sessionID)
	if err == nil && session.Project != "" {
		if completed, err := s.c.Actions.CompleteInProgress(session.Project); err == nil && completed > 0 {
			log.Printf("[pipeline] Marked %d actions as done for %s", completed, sid)
		}
	}

	log.Printf("[pipeline] Finalized session %s", sid)
	return nil
}
