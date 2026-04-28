package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"imprint/internal/config"
	"imprint/internal/server/handler"
)

// RouterDeps holds optional handler dependencies. Nil fields fall back to
// notImplemented; in production main.go always populates every field, the
// nil branches exist to keep tests usable with partial wiring.
type RouterDeps struct {
	Sessions     *handler.SessionHandler
	Observations *handler.ObservationHandler
	Memories     *handler.MemoryHandler
	Search       *handler.SearchHandler
	Graph        *handler.GraphHandler
	Actions      *handler.ActionHandler
	Advanced     *handler.AdvancedHandler
	Settings     *handler.SettingsHandler
	Pipeline     *handler.PipelineHandler
	Recall       *handler.RecallHandler
}

var startTime = time.Now()

func NewRouter(cfg *config.Config, assets embed.FS, deps *RouterDeps) chi.Router {
	if deps == nil {
		deps = &RouterDeps{}
	}
	r := chi.NewRouter()

	// Global middleware
	r.Use(CORSMiddleware(append(cfg.ViewerAllowedOrigins, "*")))
	r.Use(LoggingMiddleware)
	if cfg.Secret != "" {
		// Only apply auth to /imprint/* API routes, not static files
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/imprint/") {
					AuthMiddleware(cfg.Secret)(next).ServeHTTP(w, r)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
	}

	// Health endpoints (no auth)
	r.Get("/imprint/livez", handleLivez)
	r.Get("/imprint/health", handleHealth)

	r.Route("/imprint", func(r chi.Router) {
		// Settings
		if deps.Settings != nil {
			r.Get("/settings", deps.Settings.HandleGetSettings)
			r.Post("/settings", deps.Settings.HandleUpdateSettings)
		}

		// Sessions
		if deps.Sessions != nil {
			r.Post("/session/start", deps.Sessions.HandleStart)
			r.Post("/session/end", deps.Sessions.HandleEnd)
			r.Get("/sessions", deps.Sessions.HandleList)
		} else {
			r.Post("/session/start", notImplemented)
			r.Post("/session/end", notImplemented)
			r.Get("/sessions", notImplemented)
		}

		// Observations
		if deps.Observations != nil {
			r.Get("/observations", deps.Observations.HandleList)
			r.Get("/observations/count", deps.Observations.HandleCount)
			r.Post("/observe", deps.Observations.HandleObserve)
		} else {
			r.Get("/observations", notImplemented)
			r.Get("/observations/count", notImplemented)
			r.Post("/observe", notImplemented)
		}

		// Search + Context
		if deps.Search != nil {
			r.Post("/search", deps.Search.HandleSearch)
			r.Post("/smart-search", deps.Search.HandleSearch) // alias
			r.Post("/context", deps.Search.HandleContext)
			r.Post("/enrich", deps.Search.HandleEnrich)
		} else {
			r.Post("/search", notImplemented)
			r.Post("/smart-search", notImplemented)
			r.Post("/context", notImplemented)
			r.Post("/enrich", notImplemented)
		}

		// Recall (LLM synthesis on top of search)
		if deps.Recall != nil {
			r.Post("/recall", deps.Recall.HandleRecall)
		} else {
			r.Post("/recall", notImplemented)
		}

		// Memories
		if deps.Memories != nil {
			r.Post("/remember", deps.Memories.HandleRemember)
			r.Post("/forget", deps.Memories.HandleForget)
			r.Get("/memories", deps.Memories.HandleList)
			r.Get("/memories/concepts", deps.Memories.HandleConcepts)
			r.Get("/memories/history", deps.Memories.HandleHistory)
			r.Post("/evolve", deps.Memories.HandleEvolve)
		} else {
			r.Post("/remember", notImplemented)
			r.Post("/forget", notImplemented)
			r.Get("/memories", notImplemented)
			r.Get("/memories/concepts", notImplemented)
			r.Get("/memories/history", notImplemented)
			r.Post("/evolve", notImplemented)
		}

		// Pipeline
		if deps.Pipeline != nil {
			r.Post("/summarize", deps.Pipeline.HandleSummarize)
			r.Post("/consolidate", deps.Pipeline.HandleFullPipeline)
			r.Post("/consolidate-pipeline", deps.Pipeline.HandleConsolidatePipeline)
			r.Post("/finalize", deps.Pipeline.HandleFinalize)
			r.Get("/pipeline/status", deps.Pipeline.HandleStats)
		} else {
			r.Post("/summarize", notImplemented)
			r.Post("/consolidate", notImplemented)
			r.Post("/consolidate-pipeline", notImplemented)
			r.Post("/finalize", notImplemented)
			r.Get("/pipeline/status", notImplemented)
		}

		// Graph
		if deps.Graph != nil {
			r.Post("/graph/extract", deps.Graph.HandleExtract)
			r.Post("/graph/query", deps.Graph.HandleQuery)
			r.Get("/graph/stats", deps.Graph.HandleStats)
			r.Get("/graph/all", deps.Graph.HandleAll)
			r.Post("/relations", deps.Graph.HandleRelations)
		} else {
			r.Post("/graph/extract", notImplemented)
			r.Post("/graph/query", notImplemented)
			r.Get("/graph/stats", notImplemented)
			r.Get("/graph/all", notImplemented)
			r.Post("/relations", notImplemented)
		}

		// Actions + Leases + Routines
		if deps.Actions != nil {
			r.Post("/actions", deps.Actions.HandleCreateAction)
			r.Get("/actions", deps.Actions.HandleListActions)
			r.Post("/actions/update", deps.Actions.HandleUpdateAction)
			r.Post("/actions/from-task", deps.Actions.HandleFromTask)
			r.Post("/actions/complete-in-progress", deps.Actions.HandleCompleteInProgress)
			r.Get("/actions/get", deps.Actions.HandleGetAction)
			r.Post("/actions/edges", deps.Actions.HandleCreateEdge)
			r.Get("/frontier", deps.Actions.HandleGetFrontier)
			r.Get("/next", deps.Actions.HandleGetNext)
			r.Post("/leases/acquire", deps.Actions.HandleAcquireLease)
			r.Post("/leases/release", deps.Actions.HandleReleaseLease)
			r.Post("/leases/renew", deps.Actions.HandleRenewLease)
			r.Post("/routines", deps.Actions.HandleCreateRoutine)
			r.Get("/routines", deps.Actions.HandleListRoutines)
			r.Post("/routines/run", deps.Actions.HandleRunRoutine)
			r.Get("/routines/status", deps.Actions.HandleRoutineStatus)
		} else {
			r.Post("/actions", notImplemented)
			r.Get("/actions", notImplemented)
			r.Post("/actions/update", notImplemented)
			r.Post("/actions/from-task", notImplemented)
			r.Post("/actions/complete-in-progress", notImplemented)
			r.Get("/actions/get", notImplemented)
			r.Post("/actions/edges", notImplemented)
			r.Get("/frontier", notImplemented)
			r.Get("/next", notImplemented)
			r.Post("/leases/acquire", notImplemented)
			r.Post("/leases/release", notImplemented)
			r.Post("/leases/renew", notImplemented)
			r.Post("/routines", notImplemented)
			r.Get("/routines", notImplemented)
			r.Post("/routines/run", notImplemented)
			r.Get("/routines/status", notImplemented)
		}

		// Advanced: Signals, Checkpoints, Sentinels, Sketches, Lessons, Insights, Facets, Audit, Governance
		if deps.Advanced != nil {
			r.Post("/signals/send", deps.Advanced.HandleSendSignal)
			r.Get("/signals", deps.Advanced.HandleListSignals)
			r.Post("/checkpoints", deps.Advanced.HandleCreateCheckpoint)
			r.Post("/checkpoints/resolve", deps.Advanced.HandleResolveCheckpoint)
			r.Get("/checkpoints", deps.Advanced.HandleListCheckpoints)
			r.Post("/sentinels", deps.Advanced.HandleCreateSentinel)
			r.Get("/sentinels", deps.Advanced.HandleListSentinels)
			r.Post("/sentinels/trigger", deps.Advanced.HandleTriggerSentinel)
			r.Post("/sentinels/check", deps.Advanced.HandleCheckSentinel)
			r.Post("/sentinels/cancel", deps.Advanced.HandleCancelSentinel)
			r.Post("/sketches", deps.Advanced.HandleCreateSketch)
			r.Get("/sketches", deps.Advanced.HandleListSketches)
			r.Post("/sketches/add", deps.Advanced.HandleAddToSketch)
			r.Post("/sketches/promote", deps.Advanced.HandlePromoteSketch)
			r.Post("/sketches/discard", deps.Advanced.HandleDiscardSketch)
			r.Post("/sketches/gc", deps.Advanced.HandleGarbageCollectSketches)
			r.Post("/lessons", deps.Advanced.HandleCreateLesson)
			r.Get("/lessons", deps.Advanced.HandleListLessons)
			r.Post("/lessons/search", deps.Advanced.HandleSearchLessons)
			r.Post("/lessons/strengthen", deps.Advanced.HandleStrengthenLesson)
			r.Post("/lessons/dismiss", deps.Advanced.HandleDismissLesson)
			r.Get("/insights", deps.Advanced.HandleListInsights)
			r.Post("/insights/search", deps.Advanced.HandleSearchInsights)
			r.Post("/facets", deps.Advanced.HandleCreateFacet)
			r.Get("/facets", deps.Advanced.HandleGetFacets)
			r.Post("/facets/remove", deps.Advanced.HandleRemoveFacet)
			r.Post("/facets/query", deps.Advanced.HandleQueryFacets)
			r.Get("/facets/stats", deps.Advanced.HandleFacetStats)
			r.Get("/audit", deps.Advanced.HandleListAudit)
			r.Get("/audit/heatmap", deps.Advanced.HandleAuditHeatmap)
			r.Delete("/governance/memories", deps.Advanced.HandleGovernanceDeleteMemory)
			r.Post("/governance/bulk-delete", deps.Advanced.HandleGovernanceBulkDelete)
		} else {
			r.Post("/signals/send", notImplemented)
			r.Get("/signals", notImplemented)
			r.Post("/checkpoints", notImplemented)
			r.Post("/checkpoints/resolve", notImplemented)
			r.Get("/checkpoints", notImplemented)
			r.Post("/sentinels", notImplemented)
			r.Get("/sentinels", notImplemented)
			r.Post("/sentinels/trigger", notImplemented)
			r.Post("/sentinels/check", notImplemented)
			r.Post("/sentinels/cancel", notImplemented)
			r.Post("/sketches", notImplemented)
			r.Get("/sketches", notImplemented)
			r.Post("/sketches/add", notImplemented)
			r.Post("/sketches/promote", notImplemented)
			r.Post("/sketches/discard", notImplemented)
			r.Post("/sketches/gc", notImplemented)
			r.Post("/lessons", notImplemented)
			r.Get("/lessons", notImplemented)
			r.Post("/lessons/search", notImplemented)
			r.Post("/lessons/strengthen", notImplemented)
			r.Post("/lessons/dismiss", notImplemented)
			r.Get("/insights", notImplemented)
			r.Post("/insights/search", notImplemented)
			r.Post("/facets", notImplemented)
			r.Get("/facets", notImplemented)
			r.Post("/facets/remove", notImplemented)
			r.Post("/facets/query", notImplemented)
			r.Get("/facets/stats", notImplemented)
			r.Get("/audit", notImplemented)
			r.Get("/audit/heatmap", notImplemented)
			r.Delete("/governance/memories", notImplemented)
			r.Post("/governance/bulk-delete", notImplemented)
		}
	})

	// Serve embedded frontend (SPA fallback)
	frontendFS, err := fs.Sub(assets, "frontend/dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(frontendFS))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			if _, err := fs.Stat(frontendFS, path); err != nil {
				// SPA fallback: serve index.html for unknown routes
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return r
}

func handleLivez(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]any{
		"error": "not implemented",
		"path":  r.URL.Path,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":        "healthy",
		"uptime":        time.Since(startTime).String(),
		"uptimeSeconds": int(time.Since(startTime).Seconds()),
		"goVersion":     runtime.Version(),
		"goroutines":    runtime.NumGoroutine(),
		"memory": map[string]any{
			"allocMB": float64(memStats.Alloc) / 1024 / 1024,
			"sysMB":   float64(memStats.Sys) / 1024 / 1024,
			"numGC":   memStats.NumGC,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
