package service

import (
	"fmt"
	"log"

	"imprint/internal/store"
)

// MemoryAdminService implements the user's right over their own memory (A5):
// purge a repo, reset to cold start, and export. It also drops the affected
// documents from the search indexes so "apagar é apagar" holds end to end.
type MemoryAdminService struct {
	c         *Container
	admin     *store.AdminStore
	removeDoc func(id string) // drops one doc from BM25 + vector; nil-safe
}

func NewMemoryAdminService(c *Container, removeDoc func(id string)) *MemoryAdminService {
	return &MemoryAdminService{c: c, admin: c.Admin, removeDoc: removeDoc}
}

// PurgeProject removes every trace of a repo's memory: first the project's
// compressed-observation docs from the search indexes, then all rows.
func (s *MemoryAdminService) PurgeProject(project string) (map[string]int64, error) {
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	// Drop the project's docs from the indexes before deleting the rows.
	if s.removeDoc != nil {
		obs, err := s.c.Observations.ListCompressedByImportance(project, 0, 100000)
		if err == nil {
			for _, o := range obs {
				s.removeDoc(o.ID)
			}
		}
	}
	counts, err := s.admin.PurgeProject(project)
	if err != nil {
		return nil, err
	}
	s.c.LogAudit("memory.purge_project", project, "project", map[string]any{"counts": counts})
	log.Printf("[admin] purged project %q: %+v", project, counts)
	return counts, nil
}

// ResetAll wipes all memory back to cold start, including the search indexes.
func (s *MemoryAdminService) ResetAll() error {
	if s.removeDoc != nil {
		_ = s.c.Observations.IterateAllCompressed(func(c *store.CompressedObservationRow) error {
			s.removeDoc(c.ID)
			return nil
		})
	}
	if err := s.admin.ResetAll(); err != nil {
		return err
	}
	s.c.LogAudit("memory.reset_all", "", "global", nil)
	log.Println("[admin] full memory reset (cold start)")
	return nil
}

// Export builds a portable, versioned snapshot of a repo's memory (A5).
func (s *MemoryAdminService) Export(project string) (*store.MemoryExport, error) {
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	mems, _ := s.c.Memories.ListRefinedByProject(project, 0, 100000)
	intuitions, _ := s.c.Intuitions.ListAll(project, 100000)
	summaries, _ := s.c.Summaries.ListByProject(project, 100000)
	compressed, _ := s.c.Observations.ListCompressedByImportance(project, 0, 100000)
	return &store.MemoryExport{
		SchemaVersion: 1,
		Project:       project,
		Memories:      mems,
		Intuitions:    intuitions,
		Summaries:     summaries,
		Compressed:    compressed,
	}, nil
}

// HasProject reports whether the repo has any data (used to 404 cleanly).
func (s *MemoryAdminService) HasProject(project string) (bool, error) {
	return s.admin.HasProject(project)
}
