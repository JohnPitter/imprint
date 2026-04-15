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

	// Create pipeline components
	compressor := pipeline.NewCompressor(llmProvider)
	worker := pipeline.NewWorker(compressor, container.Observations, cfg.CompressWorkers)
	log.Printf("[app] Compression worker pool started (%d workers)", cfg.CompressWorkers)

	// Create search indexes
	bm25, err := search.NewBM25Index(cfg.DataDir)
	if err != nil {
		log.Fatalf("[app] Failed to create BM25 index: %v", err)
	}
	vectors := search.NewVectorIndex()
	searcher := search.NewHybridSearcher(bm25, vectors, cfg.BM25Weight, cfg.VectorWeight)
	log.Println("[app] Search indexes initialized")

	// Create services
	sessionSvc := service.NewSessionService(container)
	observeSvc := service.NewObserveService(container, cfg.MaxObservationsPerSession, cfg.ToolOutputMaxLen)
	observeSvc.SetCompressor(worker) // auto-compress observations in background
	rememberSvc := service.NewRememberService(container)
	searchSvc := service.NewSearchService(container, searcher)
	contextSvc := service.NewContextService(container, cfg.ContextTokenBudget)

	// Create graph components
	graphExtractor := pipeline.NewGraphExtractor(llmProvider)
	graphSvc := service.NewGraphService(container, graphExtractor)

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
	}

	// Create router and HTTP server
	router := server.NewRouter(cfg, assets, deps)
	httpSrv := server.NewHTTPServer(cfg.Port, router)
	if err := httpSrv.Start(); err != nil {
		log.Fatalf("[app] Failed to start HTTP server: %v", err)
	}
	log.Printf("[app] Imprint running on http://localhost:%d", cfg.Port)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[app] Shutting down...")

	// Stop worker pool first (drain pending jobs)
	worker.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[app] HTTP server shutdown error: %v", err)
	}

	if err := bm25.Close(); err != nil {
		log.Printf("[app] BM25 index close error: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Printf("[app] Database close error: %v", err)
	}

	log.Println("[app] Shutdown complete")
}
