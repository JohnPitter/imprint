package service

import (
	"context"
	"log"
	"sync"
	"time"
)

// Decay configuration. We don't expose these as config — the values are
// conservative enough that surfacing them in env vars would just be a
// footgun. Memories below `decayStrengthThreshold` strength that are older
// than `decayMaxAgeDays` get soft-deleted so retrieval stays focused on
// signal. Strong memories (4+) survive forever; recent memories survive
// regardless of strength.
const (
	decayStrengthThreshold = 3
	decayMaxAgeDays        = 30
	decayRunInterval       = 6 * time.Hour
)

// Scheduler runs the pipeline periodically for active sessions.
// It ticks at the configured interval and processes summarize + consolidate
// for each active session, keeping memories up-to-date during the session
// instead of deferring everything to the expensive session-end hook.
type Scheduler struct {
	pipeline *PipelineService
	sessions *SessionService
	interval time.Duration

	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
	doneCh      chan struct{}
	lastDecayAt time.Time
}

// NewScheduler creates a new Scheduler. If intervalMin <= 0, the scheduler is disabled.
func NewScheduler(pipeline *PipelineService, sessions *SessionService, intervalMin int) *Scheduler {
	interval := time.Duration(intervalMin) * time.Minute
	return &Scheduler{
		pipeline: pipeline,
		sessions: sessions,
		interval: interval,
	}
}

// Start begins the periodic ticker. No-op if interval is 0 or already running.
func (s *Scheduler) Start() {
	if s.interval <= 0 {
		log.Println("[scheduler] Disabled (interval = 0)")
		return
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	log.Printf("[scheduler] Started with interval %s", s.interval)

	go s.loop()
}

// Stop signals the scheduler to stop and waits for it to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	<-s.doneCh
	log.Println("[scheduler] Stopped")
}

func (s *Scheduler) loop() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	// Find all active sessions
	active, err := s.pipeline.c.Sessions.GetActive()
	if err != nil {
		log.Printf("[scheduler] Failed to list active sessions: %v", err)
		return
	}

	if len(active) == 0 {
		return
	}

	log.Printf("[scheduler] Tick: %d active session(s)", len(active))

	for _, session := range active {
		sid := session.ID
		sidShort := sid[:min(12, len(sid))]

		// Check if there are enough compressed observations to process
		obs, err := s.pipeline.c.Observations.ListCompressed(sid, 1, 0)
		if err != nil || len(obs) == 0 {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

		// 1. Summarize (generates/updates session summary)
		if _, err := s.pipeline.Summarize(ctx, sid); err != nil {
			log.Printf("[scheduler] Summarize failed for %s: %v", sidShort, err)
		}

		// 2. Consolidate (memories + lessons from patterns)
		if _, err := s.pipeline.Consolidate(ctx, sid); err != nil {
			log.Printf("[scheduler] Consolidate failed for %s: %v", sidShort, err)
		}

		// 3. Extract actions (in_progress for active sessions)
		if _, err := s.pipeline.ExtractActions(ctx, sid); err != nil {
			log.Printf("[scheduler] ExtractActions failed for %s: %v", sidShort, err)
		}

		cancel()
	}

	// Decay sweep: low-strength memories older than the cutoff get
	// soft-deleted. Runs at most once every decayRunInterval so we don't
	// thrash the DB with UPDATEs every 5 minutes — once every few hours
	// is plenty for a TTL of 30 days.
	if time.Since(s.lastDecayAt) >= decayRunInterval {
		n, err := s.pipeline.c.Memories.DecayOld(decayStrengthThreshold, decayMaxAgeDays)
		if err != nil {
			log.Printf("[scheduler] Decay failed: %v", err)
		} else if n > 0 {
			log.Printf("[scheduler] Decay: archived %d low-strength memories older than %dd", n, decayMaxAgeDays)
		}
		s.lastDecayAt = time.Now()
	}
}
