package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"imprint/internal/config"
	"imprint/internal/eventbus"
	"imprint/internal/llm"
	"imprint/internal/pipeline"
	"imprint/internal/search"
	"imprint/internal/server"
	"imprint/internal/server/handler"
	"imprint/internal/service"
	"imprint/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

// Versão e commit injetados via -ldflags no build oficial:
//
//	go build -ldflags="-X main.version=1.5.0 -X main.commit=$(git rev-parse --short HEAD)"
//
// Em builds locais sem ldflags, ficam como "dev" — sinaliza pro user que
// o binário não veio de release oficial.
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Repassa a versão pro pacote server pra o handler /health expor.
	server.SetVersion(version, commit)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[app] Failed to load config: %v", err)
	}
	log.Printf("[app] Config loaded, data dir: %s", cfg.DataDir)

	// Apply user settings (from ~/.imprint/settings.json)
	userSettings, err := config.LoadUserSettings(cfg.DataDir)
	if err != nil {
		log.Printf("[app] Warning: could not load user settings: %v", err)
	} else {
		config.ApplyUserSettings(cfg, userSettings)
	}

	if cfg.AnthropicAuthMode == "oauth" {
		log.Println("[app] Anthropic auth: Claude Code OAuth token (auto-detected)")
	} else if cfg.AnthropicAPIKey != "" {
		log.Println("[app] Anthropic auth: API key")
	}
	if cfg.CodexOAuthAvailable {
		log.Printf("[app] Codex ChatGPT-OAuth detected (~/.codex/auth.json) · model %s", cfg.OpenAIOAuthModel)
	}
	if cfg.OpenAIAuthMode == "codex" {
		log.Printf("[app] OpenAI auth: Codex API key (auto-detected) · model %s", cfg.OpenAIModel)
	} else if cfg.OpenAIAPIKey != "" {
		log.Printf("[app] OpenAI auth: API key · model %s", cfg.OpenAIModel)
	}

	// Open database
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("[app] Failed to open database: %v", err)
	}
	log.Println("[app] Database opened successfully")

	// Build LLM provider chain
	llmProvider := llm.BuildProviderChain(cfg)
	log.Printf("[app] LLM provider: %s (available: %v)", llmProvider.Name(), llmProvider.Available())

	// Create service container
	container := service.NewContainer(db)

	// Token economy budget ceiling (Phase 1): protects *before* spend. When a cap
	// is hit, instrumented Haiku calls are skipped and injection falls to the
	// minimum, without breaking the main path.
	llm.GlobalBudget.SetLimits(cfg.MaxHaikuTokensPerSession, cfg.MaxHaikuTokensPerDay)
	log.Printf("[app] Token budget: %d/session, %d/day (0 = unlimited)", cfg.MaxHaikuTokensPerSession, cfg.MaxHaikuTokensPerDay)

	// Wire the token economy ledger (Phase 1). The LLM layer reports every
	// instrumented Haiku call here so spend is attributed per session/repo. The
	// sink is best-effort and must never block the LLM path (invariant 6).
	llm.SpendSink = func(e llm.SpendEvent) {
		container.Ledger.AppendSpend(store.SpendEntry{
			SpendPoint:   e.SpendPoint,
			Provider:     e.Provider,
			SessionID:    e.SessionID,
			Project:      e.Project,
			InputTokens:  e.InputTokens,
			OutputTokens: e.OutputTokens,
		})
	}

	// Attach file-based write-ahead log for crash recovery and poisoning detection.
	wal, err := store.NewWAL(cfg.DataDir)
	if err != nil {
		log.Printf("[app] Warning: could not create WAL: %v (continuing without file-based audit)", err)
	} else {
		container.WAL = wal
		log.Println("[app] Write-ahead log initialized")
	}

	// Create search indexes (must come before workers so they can index as they compress)
	bm25, err := search.NewBM25Index(cfg.DataDir)
	if err != nil {
		log.Fatalf("[app] Failed to create BM25 index: %v", err)
	}
	vectors := search.NewVectorIndex()
	searcher := search.NewHybridSearcher(bm25, vectors, cfg.BM25Weight, cfg.VectorWeight)
	log.Println("[app] Search indexes initialized")

	// Backfill BM25 index if empty but DB has compressed observations.
	// This catches existing installations where indexing was previously broken.
	if indexed, err := bm25.Count(); err == nil && indexed == 0 {
		if dbCount, _ := container.Observations.CountAllCompressed(); dbCount > 0 {
			log.Printf("[app] Empty BM25 index with %d compressed obs in DB — backfilling…", dbCount)
			go backfillBM25(container.Observations, bm25)
		}
	}

	// Create pipeline components
	compressor := pipeline.NewCompressor(llmProvider, cfg.ExtractionMode)
	compressor.SetImportanceFilter(cfg.CompressFilterEnabled, cfg.CompressMinImportance)
	log.Printf("[app] Extraction mode: %s · importance filter: %v (min %d)", cfg.ExtractionMode, cfg.CompressFilterEnabled, cfg.CompressMinImportance)
	worker := pipeline.NewWorkerWithIndex(compressor, container.Observations, bm25, cfg.CompressWorkers)
	worker.SetCrediter(container.Ledger) // Phase 1 "memory used" signal
	log.Printf("[app] Compression worker pool started (%d workers)", cfg.CompressWorkers)

	// Create services
	contextSvc := service.NewContextService(container, cfg.ContextTokenBudget)
	contextSvc.SetDataDir(cfg.DataDir)
	contextSvc.SetInjectionCap(cfg.MaxInjectionTokens)
	sessionSvc := service.NewSessionService(container, contextSvc)
	sessionTracker := service.NewSessionTracker()
	observeSvc := service.NewObserveService(container, cfg.MaxObservationsPerSession, cfg.ToolOutputMaxLen)
	observeSvc.SetCompressor(worker) // auto-compress observations in background
	rememberSvc := service.NewRememberService(container)
	searchSvc := service.NewSearchService(container, searcher)

	// Create graph components
	graphExtractor := pipeline.NewGraphExtractor(llmProvider)
	graphSvc := service.NewGraphService(container, graphExtractor)

	// Phase 4: wire the graph blast radius into lazy injection as a relevance
	// signal — editing a file also surfaces memories about structurally related files.
	if cfg.BlastRadiusDepth > 0 {
		contextSvc.SetBlastRadius(func(file string, depth int) []string {
			files, _ := graphSvc.BlastRadius(file, depth)
			return files
		}, cfg.BlastRadiusDepth)
	}

	// Create pipeline services
	summarizer := pipeline.NewSummarizer(llmProvider)
	consolidator := pipeline.NewConsolidator(llmProvider)
	reflector := pipeline.NewReflector(llmProvider)
	pipelineSvc := service.NewPipelineService(container, summarizer, consolidator, reflector, graphSvc)

	// Intuition (rooted layer, Phase 2): convergence detection + auto-weakening.
	rooter := pipeline.NewRooter(llmProvider)
	intuitionSvc := service.NewIntuitionService(container, rooter, service.IntuitionConfig{
		MinStrength:      cfg.IntuitionMinStrength,
		MinConvergence:   cfg.IntuitionMinConvergence,
		MinSessions:      cfg.IntuitionMinSessions,
		MaxActive:        cfg.IntuitionMaxActive,
		ContradictionHit: cfg.IntuitionContradictionHit,
		DemoteFloor:      cfg.IntuitionDemoteFloor,
	})
	contextSvc.SetIntuitionMax(cfg.IntuitionMaxActive)

	// Memory governance (A5): purge/reset/export, dropping docs from the indexes.
	memoryAdminSvc := service.NewMemoryAdminService(container, func(id string) {
		_ = bm25.Remove(id)
		vectors.Remove(id)
	})

	// Create advanced services
	actionSvc := service.NewActionService(container)
	advancedSvc := service.NewAdvancedService(container)

	// Wire the kanban-change pubsub: every action mutation publishes to the
	// bus so SSE subscribers can refresh the kanban without waiting for the
	// next poll. The publish is fire-and-forget and never blocks the store.
	actionsBus := eventbus.New()
	container.Actions.SetOnChange(func() { actionsBus.Publish("actions:changed") })

	// Recall: searches via SearchService, then asks the LLM to synthesise.
	recallSvc := service.NewRecallService(searchSvc, llmProvider)

	// Create handlers
	deps := &server.RouterDeps{
		Sessions:     handler.NewSessionHandler(sessionSvc, container, sessionTracker),
		Observations: handler.NewObservationHandler(observeSvc),
		Memories:     handler.NewMemoryHandler(rememberSvc, llmProvider),
		Search:       handler.NewSearchHandler(searchSvc, contextSvc, container.Eval),
		Graph:        handler.NewGraphHandler(graphSvc),
		Actions:      handler.NewActionHandler(actionSvc, actionsBus),
		Advanced:     handler.NewAdvancedHandler(advancedSvc),
		Settings:     handler.NewSettingsHandler(cfg),
		Pipeline:     handler.NewPipelineHandler(pipelineSvc),
		Recall:       handler.NewRecallHandler(recallSvc),
		Economy:      handler.NewEconomyHandler(container.Ledger, newEconomyConfig(cfg)),
		Intuitions:   handler.NewIntuitionHandler(intuitionSvc),
		MemoryAdmin:  handler.NewMemoryAdminHandler(contextSvc, memoryAdminSvc, cfg.LazyInjectMax),
	}

	// Create router and HTTP server
	router := server.NewRouter(cfg, assets, deps)
	httpSrv := server.NewHTTPServer(cfg.Port, router)
	if err := httpSrv.Start(); err != nil {
		log.Fatalf("[app] Failed to start HTTP server: %v", err)
	}
	log.Printf("[app] Imprint running on http://localhost:%d", cfg.Port)

	// Start background pipeline scheduler
	scheduler := service.NewScheduler(pipelineSvc, sessionSvc, sessionTracker, cfg, cfg.PipelineIntervalMin)
	scheduler.SetIntuition(intuitionSvc)
	scheduler.Start()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[app] Shutting down...")

	// Stop scheduler and worker pool first (drain pending jobs)
	scheduler.Stop()
	worker.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[app] HTTP server shutdown error: %v", err)
	}

	if err := bm25.Close(); err != nil {
		log.Printf("[app] BM25 index close error: %v", err)
	}

	if container.WAL != nil {
		if err := container.WAL.Close(); err != nil {
			log.Printf("[app] WAL close error: %v", err)
		}
	}

	if err := db.Close(); err != nil {
		log.Printf("[app] Database close error: %v", err)
	}

	log.Println("[app] Shutdown complete")
}

// resolvePlan determines the billing plan for the economy display. An explicit
// IMPRINT_PLAN wins; otherwise an API key implies the pay-per-token "api" plan
// and Claude Code OAuth implies a subscription ("pro").
func resolvePlan(cfg *config.Config) string {
	switch cfg.Plan {
	case "api", "pro", "max":
		return cfg.Plan
	}
	// Subscription logins (Claude OAuth, Codex/ChatGPT OAuth) → "pro" so the UI
	// shows breathing room (fôlego) instead of currency.
	if cfg.AnthropicAuthMode == "oauth" || (cfg.CodexOAuthAvailable && cfg.AnthropicAPIKey == "") {
		return "pro"
	}
	return "api"
}

// newEconomyConfig wires the economy meter to the pricing of whatever model does
// the background work: GPT-5 prices when OpenAI is the active backend (Codex),
// Haiku prices otherwise. The active backend is OpenAI when it is configured and
// Anthropic is not.
func newEconomyConfig(cfg *config.Config) handler.EconomyConfig {
	inPrice, outPrice := cfg.HaikuPriceInPerMTok, cfg.HaikuPriceOutPerMTok
	if cfg.OpenAIAPIKey != "" && cfg.AnthropicAPIKey == "" {
		inPrice, outPrice = cfg.OpenAIPriceInPerMTok, cfg.OpenAIPriceOutPerMTok
	}
	return handler.EconomyConfig{
		Plan:            resolvePlan(cfg),
		PriceInPerMTok:  inPrice,
		PriceOutPerMTok: outPrice,
	}
}

// backfillBM25 reindexes every compressed observation in the DB into the BM25
// index. Used on first run after the indexing-on-compress fix to catch up on
// observations created while indexing was broken.
func backfillBM25(obs *store.ObservationStore, idx *search.BM25Index) {
	indexed := 0
	skipped := 0
	err := obs.IterateAllCompressed(func(c *store.CompressedObservationRow) error {
		narrative := ""
		if c.Narrative != nil {
			narrative = *c.Narrative
		}
		doc := search.IndexDocument{
			ID:        c.ID,
			SessionID: c.SessionID,
			Title:     c.Title,
			Narrative: narrative,
			Facts:     joinSpace(c.Facts),
			Concepts:  joinSpace(c.Concepts),
			Files:     joinSpace(c.Files),
			Type:      c.Type,
		}
		if err := idx.Index(doc); err != nil {
			skipped++
			return nil // keep going on individual failures
		}
		indexed++
		return nil
	})
	if err != nil {
		log.Printf("[app] Backfill iterate error: %v", err)
		return
	}
	log.Printf("[app] BM25 backfill complete: %d indexed, %d skipped", indexed, skipped)
}

func joinSpace(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += " "
		}
		out += v
	}
	return out
}
