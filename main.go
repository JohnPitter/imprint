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

func main() {
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
	compressor := pipeline.NewCompressor(llmProvider)
	worker := pipeline.NewWorkerWithIndex(compressor, container.Observations, bm25, cfg.CompressWorkers)
	log.Printf("[app] Compression worker pool started (%d workers)", cfg.CompressWorkers)

	// Create services
	contextSvc := service.NewContextService(container, cfg.ContextTokenBudget)
	contextSvc.SetDataDir(cfg.DataDir)
	sessionSvc := service.NewSessionService(container, contextSvc)
	observeSvc := service.NewObserveService(container, cfg.MaxObservationsPerSession, cfg.ToolOutputMaxLen)
	observeSvc.SetCompressor(worker) // auto-compress observations in background
	rememberSvc := service.NewRememberService(container)
	searchSvc := service.NewSearchService(container, searcher)

	// Create graph components
	graphExtractor := pipeline.NewGraphExtractor(llmProvider)
	graphSvc := service.NewGraphService(container, graphExtractor)

	// Create pipeline services
	summarizer := pipeline.NewSummarizer(llmProvider)
	consolidator := pipeline.NewConsolidator(llmProvider)
	reflector := pipeline.NewReflector(llmProvider)
	pipelineSvc := service.NewPipelineService(container, summarizer, consolidator, reflector, graphSvc)

	// Create advanced services
	actionSvc := service.NewActionService(container)
	advancedSvc := service.NewAdvancedService(container)

	// Create handlers
	deps := &server.RouterDeps{
		Sessions:     handler.NewSessionHandler(sessionSvc),
		Observations: handler.NewObservationHandler(observeSvc),
		Memories:     handler.NewMemoryHandler(rememberSvc),
		Search:       handler.NewSearchHandler(searchSvc, contextSvc),
		Graph:        handler.NewGraphHandler(graphSvc),
		Actions:      handler.NewActionHandler(actionSvc),
		Advanced:     handler.NewAdvancedHandler(advancedSvc),
		Settings:     handler.NewSettingsHandler(cfg),
		Pipeline:     handler.NewPipelineHandler(pipelineSvc),
	}

	// Create router and HTTP server
	router := server.NewRouter(cfg, assets, deps)
	httpSrv := server.NewHTTPServer(cfg.Port, router)
	if err := httpSrv.Start(); err != nil {
		log.Fatalf("[app] Failed to start HTTP server: %v", err)
	}
	log.Printf("[app] Imprint running on http://localhost:%d", cfg.Port)

	// Start background pipeline scheduler
	scheduler := service.NewScheduler(pipelineSvc, sessionSvc, cfg.PipelineIntervalMin)
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
