package service

import (
	"context"
	"log"
	"sync"
	"time"

	"imprint/internal/config"
)

// decayRunInterval mantém-se hardcoded — ele controla apenas a frequência
// do sweep, não a política. 6h é suficiente pra qualquer TTL de dias.
// Os thresholds reais (strength + age) vêm de cfg.DecayMinStrength /
// cfg.DecayMaxAgeDays e podem ser editados via Settings UI.
const decayRunInterval = 6 * time.Hour

// Scheduler runs the pipeline periodically for active sessions.
// It ticks at the configured interval and processes summarize + consolidate
// for each active session, keeping memories up-to-date during the session
// instead of deferring everything to the expensive session-end hook.
type Scheduler struct {
	pipeline *PipelineService
	sessions *SessionService
	cfg      *config.Config
	interval time.Duration

	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
	doneCh      chan struct{}
	lastDecayAt time.Time
}

// NewScheduler creates a new Scheduler. If intervalMin <= 0, the scheduler is disabled.
// O cfg é a referência viva — re-lemos dele a cada tick pra que mudanças via
// Settings UI tenham efeito sem reiniciar o servidor.
func NewScheduler(pipeline *PipelineService, sessions *SessionService, cfg *config.Config, intervalMin int) *Scheduler {
	interval := time.Duration(intervalMin) * time.Minute
	return &Scheduler{
		pipeline: pipeline,
		sessions: sessions,
		cfg:      cfg,
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

		// 4. Extract knowledge graph entities (incremental, idempotente).
		// Limite baixo pra não disparar em backlog histórico — em sessão
		// ativa típica com produção de ~1 obs/min, drena de sobra.
		if _, err := s.pipeline.ExtractGraph(ctx, sid, 5); err != nil {
			log.Printf("[scheduler] ExtractGraph failed for %s: %v", sidShort, err)
		}

		cancel()
	}

	// Decay sweep: low-strength memories older than the cutoff get
	// soft-deleted. Runs at most once every decayRunInterval; thresholds
	// vêm do cfg vivo (editáveis via Settings UI sem restart).
	if time.Since(s.lastDecayAt) >= decayRunInterval {
		minStrength := s.cfg.DecayMinStrength
		maxAgeDays := s.cfg.DecayMaxAgeDays
		// Defesa em profundidade: se o user setou 0/negativo via UI, usar
		// defaults conservadores em vez de "decay everything imediato".
		if minStrength <= 0 {
			minStrength = 3
		}
		if maxAgeDays <= 0 {
			maxAgeDays = 30
		}
		n, err := s.pipeline.c.Memories.DecayOld(minStrength, maxAgeDays)
		if err != nil {
			log.Printf("[scheduler] Decay failed: %v", err)
		} else if n > 0 {
			log.Printf("[scheduler] Decay: archived %d memories (strength<=%d, age>=%dd)", n, minStrength, maxAgeDays)
		}
		s.lastDecayAt = time.Now()
	}
}
