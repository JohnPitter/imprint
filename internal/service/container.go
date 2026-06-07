package service

import "imprint/internal/store"

// Container holds all stores and is passed to services and handlers.
type Container struct {
	Sessions     *store.SessionStore
	Observations *store.ObservationStore
	Memories     *store.MemoryStore
	Summaries    *store.SummaryStore
	Graph        *store.GraphStore
	Actions      *store.ActionStore
	Leases       *store.LeaseStore
	Routines     *store.RoutineStore
	Signals      *store.SignalStore
	Checkpoints  *store.CheckpointStore
	Sentinels    *store.SentinelStore
	Sketches     *store.SketchStore
	Crystals     *store.CrystalStore
	Lessons      *store.LessonStore
	Insights     *store.InsightStore
	Facets       *store.FacetStore
	Audit        *store.AuditStore
	Eval         *store.EvalStore
	Ledger       *store.LedgerStore    // token economy ledger (Phase 1, nil-safe)
	Intuitions   *store.IntuitionStore // rooted memory layer (Phase 2)
	Admin        *store.AdminStore     // forget/export/reset (A5)
	WAL          *store.WAL            // file-based write-ahead log (optional, nil-safe)
}

// NewContainer creates a Container with all stores initialized from the given DB.
func NewContainer(db *store.DB) *Container {
	return &Container{
		Sessions:     store.NewSessionStore(db),
		Observations: store.NewObservationStore(db),
		Memories:     store.NewMemoryStore(db),
		Summaries:    store.NewSummaryStore(db),
		Graph:        store.NewGraphStore(db),
		Actions:      store.NewActionStore(db),
		Leases:       store.NewLeaseStore(db),
		Routines:     store.NewRoutineStore(db),
		Signals:      store.NewSignalStore(db),
		Checkpoints:  store.NewCheckpointStore(db),
		Sentinels:    store.NewSentinelStore(db),
		Sketches:     store.NewSketchStore(db),
		Crystals:     store.NewCrystalStore(db),
		Lessons:      store.NewLessonStore(db),
		Insights:     store.NewInsightStore(db),
		Facets:       store.NewFacetStore(db),
		Audit:        store.NewAuditStore(db),
		Eval:         store.NewEvalStore(db),
		Ledger:       store.NewLedgerStore(db),
		Intuitions:   store.NewIntuitionStore(db),
		Admin:        store.NewAdminStore(db),
	}
}
