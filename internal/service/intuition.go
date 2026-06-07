package service

import (
	"context"
	"encoding/json"
	"log"

	"imprint/internal/pipeline"
	"imprint/internal/store"

	"github.com/google/uuid"
)

// Rooter is the LLM-backed synthesis the intuition service depends on. An
// interface so the convergence/contradiction policy can be unit-tested with a
// fake (the real one is *pipeline.Rooter).
type Rooter interface {
	Synthesize(ctx context.Context, project, sessionID string, cluster []store.MemoryRow) (*pipeline.RootedDraft, error)
	Contradicts(ctx context.Context, project, statement string, mem store.MemoryRow) (bool, string, error)
}

// IntuitionConfig holds the (deliberately high) birth bar and weakening policy.
type IntuitionConfig struct {
	MinStrength      int     // refined memories considered
	MinConvergence   int     // min converging insights in a cluster
	MinSessions      int     // distinct sessions a cluster must span
	MaxActive        int     // hard residency cap per repo
	ContradictionHit float64 // strength drop per contradiction
	DemoteFloor      float64 // strength at/below which an intuition demotes
}

// IntuitionService owns the rooted layer lifecycle: detection→rooting (only
// automatic, by convergence — 3.5), contradiction→auto-weakening, and the
// inspection/CRUD surface (invariant 11).
type IntuitionService struct {
	c      *Container
	rooter Rooter
	cfg    IntuitionConfig
}

func NewIntuitionService(c *Container, rooter Rooter, cfg IntuitionConfig) *IntuitionService {
	if cfg.MinConvergence < 2 {
		cfg.MinConvergence = 2
	}
	if cfg.MaxActive < 1 {
		cfg.MaxActive = 5
	}
	return &IntuitionService{c: c, rooter: rooter, cfg: cfg}
}

// DetectAndRoot scans the project's refined memories for convergent clusters and
// roots an intuition from each that clears the bar. Rooting is the ONLY way an
// intuition is born (3.5) — never by hand. Returns the newly rooted intuitions.
func (s *IntuitionService) DetectAndRoot(ctx context.Context, project string) ([]store.IntuitionRow, error) {
	if project == "" || s.rooter == nil {
		return nil, nil
	}
	mems, err := s.c.Memories.ListRefinedByProject(project, s.cfg.MinStrength, 200)
	if err != nil {
		return nil, err
	}
	if len(mems) < s.cfg.MinConvergence {
		return nil, nil // not enough material to converge — cold start stays neutral
	}

	active, _ := s.c.Intuitions.ListActive(project, s.cfg.MaxActive*4)

	var created []store.IntuitionRow
	for _, cluster := range clusterMemoriesByConcept(mems) {
		if len(cluster) < s.cfg.MinConvergence {
			continue
		}
		if distinctSessions(cluster) < s.cfg.MinSessions {
			continue // must span distinct episodes, not one recurring session
		}
		clusterConcepts := unionConcepts(cluster)

		// Already covered? Reinforce the existing intuition instead of duplicating.
		if existing := coveringIntuition(active, clusterConcepts); existing != nil {
			_ = s.c.Intuitions.Reinforce(existing.ID, 0.5)
			continue
		}

		draft, err := s.rooter.Synthesize(ctx, project, "", cluster)
		if err != nil {
			log.Printf("[intuition] synthesize error (project=%s): %v", project, err)
			continue
		}
		if draft == nil {
			continue // did not clear the convergence bar
		}

		// Enforce the hard residency cap: make room by demoting the weakest.
		if n, _ := s.c.Intuitions.CountActive(project); n >= s.cfg.MaxActive {
			if weak, _ := s.c.Intuitions.WeakestActive(project); weak != nil {
				_ = s.c.Intuitions.SetStatus(weak.ID, "demoted")
			}
		}

		row := &store.IntuitionRow{
			ID:            "intu_" + uuid.New().String()[:8],
			Project:       project,
			Statement:     draft.Statement,
			Strength:      6,
			EvidenceIDs:   memoryIDs(cluster),
			EvidenceCount: len(cluster),
			Concepts:      draft.Concepts,
			Files:         draft.Files,
		}
		if err := s.c.Intuitions.Create(row); err != nil {
			log.Printf("[intuition] create error: %v", err)
			continue
		}
		s.c.LogAudit("intuition.root", row.ID, "intuition", map[string]any{"project": project, "evidence": len(cluster)})
		created = append(created, *row)
		log.Printf("[intuition] rooted (project=%s, evidence=%d): %s", project, len(cluster), draft.Statement)
	}
	return created, nil
}

// CheckContradictions tests the project's active intuitions against newly formed
// refined memories. A contradiction lowers the intuition's strength and, past
// the floor, demotes it back to refined (3.5 / 3.6). Each contradiction is
// logged (invariant 11). Bounded LLM use: one check per (intuition, first
// concept-overlapping new memory).
func (s *IntuitionService) CheckContradictions(ctx context.Context, project string, newMems []store.MemoryRow) error {
	if project == "" || s.rooter == nil || len(newMems) == 0 {
		return nil
	}
	active, err := s.c.Intuitions.ListActive(project, s.cfg.MaxActive)
	if err != nil {
		return err
	}
	for _, intu := range active {
		intuConcepts := toSet(intu.Concepts)
		for _, mem := range newMems {
			if !overlaps(intuConcepts, jsonToStrings(mem.Concepts)) {
				continue
			}
			// Skip pairs already judged — avoids re-spending an LLM call and
			// double-penalizing the same memory across background passes.
			if seen, _ := s.c.Intuitions.ContradictionExists(intu.ID, mem.ID); seen {
				continue
			}
			contradicts, detail, err := s.rooter.Contradicts(ctx, project, intu.Statement, mem)
			if err != nil {
				log.Printf("[intuition] contradiction check error: %v", err)
				break
			}
			if contradicts {
				newStrength, demoted, err := s.c.Intuitions.RecordContradiction(
					intu.ID, mem.ID, detail, s.cfg.ContradictionHit, s.cfg.DemoteFloor)
				if err != nil {
					log.Printf("[intuition] record contradiction error: %v", err)
				} else {
					s.c.LogAudit("intuition.contradicted", intu.ID, "intuition",
						map[string]any{"memory": mem.ID, "strength": newStrength, "demoted": demoted})
					log.Printf("[intuition] contradicted (id=%s, strength→%.1f, demoted=%v): %s",
						intu.ID, newStrength, demoted, detail)
				}
				break // one contradiction per intuition per pass is enough
			}
		}
	}
	return nil
}

// ListActive returns the resident intuitions for a project (for injection).
func (s *IntuitionService) ListActive(project string, limit int) ([]store.IntuitionRow, error) {
	return s.c.Intuitions.ListActive(project, limit)
}

// ListAll returns intuitions of any status (inspection screen).
func (s *IntuitionService) ListAll(project string, limit int) ([]store.IntuitionRow, error) {
	return s.c.Intuitions.ListAll(project, limit)
}

// Contradictions returns the contradiction log for one intuition.
func (s *IntuitionService) Contradictions(id string, limit int) ([]store.Contradiction, error) {
	return s.c.Intuitions.Contradictions(id, limit)
}

// Demote manually demotes an intuition back to refined (user override).
func (s *IntuitionService) Demote(id string) error {
	if err := s.c.Intuitions.SetStatus(id, "demoted"); err != nil {
		return err
	}
	s.c.LogAudit("intuition.demote", id, "intuition", map[string]any{"by": "user"})
	return nil
}

// Delete permanently removes an intuition (user override; A5).
func (s *IntuitionService) Delete(id string) error {
	if err := s.c.Intuitions.HardDelete(id); err != nil {
		return err
	}
	s.c.LogAudit("intuition.delete", id, "intuition", map[string]any{"by": "user"})
	return nil
}

// ── clustering helpers ───────────────────────────────────────────────────────

// clusterMemoriesByConcept groups memories that share at least one concept,
// via union-find (same approach as the consolidation pipeline).
func clusterMemoriesByConcept(mems []store.MemoryRow) [][]store.MemoryRow {
	n := len(mems)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	conceptIdx := map[string][]int{}
	for i, m := range mems {
		for _, c := range jsonToStrings(m.Concepts) {
			conceptIdx[c] = append(conceptIdx[c], i)
		}
	}
	for _, idxs := range conceptIdx {
		for j := 1; j < len(idxs); j++ {
			union(idxs[0], idxs[j])
		}
	}
	groups := map[int][]store.MemoryRow{}
	for i := range mems {
		r := find(i)
		groups[r] = append(groups[r], mems[i])
	}
	out := make([][]store.MemoryRow, 0, len(groups))
	for _, g := range groups {
		out = append(out, g)
	}
	return out
}

func distinctSessions(cluster []store.MemoryRow) int {
	seen := map[string]struct{}{}
	for _, m := range cluster {
		var sids []string
		_ = json.Unmarshal(m.SessionIDs, &sids)
		for _, sid := range sids {
			if sid != "" {
				seen[sid] = struct{}{}
			}
		}
	}
	return len(seen)
}

func unionConcepts(cluster []store.MemoryRow) []string {
	set := map[string]struct{}{}
	for _, m := range cluster {
		for _, c := range jsonToStrings(m.Concepts) {
			if c != "" {
				set[c] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func memoryIDs(cluster []store.MemoryRow) []string {
	out := make([]string, 0, len(cluster))
	for _, m := range cluster {
		out = append(out, m.ID)
	}
	return out
}

// coveringIntuition returns an active intuition whose concepts substantially
// overlap the cluster (>= 2 shared concepts), or nil.
func coveringIntuition(active []store.IntuitionRow, clusterConcepts []string) *store.IntuitionRow {
	cc := toSet(clusterConcepts)
	for i := range active {
		shared := 0
		for _, c := range active[i].Concepts {
			if _, ok := cc[c]; ok {
				shared++
			}
		}
		if shared >= 2 {
			return &active[i]
		}
	}
	return nil
}

func toSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		if it != "" {
			m[it] = struct{}{}
		}
	}
	return m
}

func overlaps(set map[string]struct{}, items []string) bool {
	for _, it := range items {
		if _, ok := set[it]; ok {
			return true
		}
	}
	return false
}
