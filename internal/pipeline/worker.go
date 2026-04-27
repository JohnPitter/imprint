package pipeline

import (
	"context"
	"log"
	"strings"
	"sync"

	"imprint/internal/search"
	"imprint/internal/store"
)

// Job represents a compression job for a raw observation.
type Job struct {
	Raw *store.RawObservationRow
}

// Indexer is the subset of the BM25 index used by the worker. Defined as an
// interface so the worker can be tested without a real Bleve index.
type Indexer interface {
	Index(doc search.IndexDocument) error
}

// Worker is a background worker pool for processing LLM pipeline jobs.
type Worker struct {
	compressor *Compressor
	obsStore   *store.ObservationStore
	indexer    Indexer // optional; if nil, compressed observations are not indexed
	jobs       chan Job
	wg         sync.WaitGroup
	cancel     context.CancelFunc
}

// NewWorker creates a new worker pool with numWorkers goroutines.
func NewWorker(compressor *Compressor, obsStore *store.ObservationStore, numWorkers int) *Worker {
	return NewWorkerWithIndex(compressor, obsStore, nil, numWorkers)
}

// NewWorkerWithIndex creates a worker pool that also indexes compressed
// observations into the provided BM25 index for full-text search.
func NewWorkerWithIndex(compressor *Compressor, obsStore *store.ObservationStore, indexer Indexer, numWorkers int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		compressor: compressor,
		obsStore:   obsStore,
		indexer:    indexer,
		jobs:       make(chan Job, 100),
		cancel:     cancel,
	}

	for i := range numWorkers {
		w.wg.Add(1)
		go w.run(ctx, i)
	}

	return w
}

// Submit adds a job to the queue. Drops the observation if the queue is full.
func (w *Worker) Submit(raw *store.RawObservationRow) {
	select {
	case w.jobs <- Job{Raw: raw}:
	default:
		log.Printf("[pipeline] Job queue full, dropping observation %s", raw.ID)
	}
}

// Stop gracefully stops all workers and waits for them to finish.
func (w *Worker) Stop() {
	w.cancel()
	close(w.jobs)
	w.wg.Wait()
	log.Println("[pipeline] Worker pool stopped")
}

func (w *Worker) run(ctx context.Context, id int) {
	defer w.wg.Done()
	for {
		select {
		case job, ok := <-w.jobs:
			if !ok {
				return
			}
			compressed, err := w.compressor.Compress(ctx, job.Raw)
			if err != nil {
				log.Printf("[pipeline] Worker %d compress error: %v", id, err)
				continue
			}
			if err := w.obsStore.CreateCompressed(compressed); err != nil {
				log.Printf("[pipeline] Worker %d store error: %v", id, err)
				continue
			}
			if w.indexer != nil {
				narrative := ""
				if compressed.Narrative != nil {
					narrative = *compressed.Narrative
				}
				doc := search.IndexDocument{
					ID:        compressed.ID,
					SessionID: compressed.SessionID,
					Title:     compressed.Title,
					Narrative: narrative,
					Facts:     strings.Join(compressed.Facts, " "),
					Concepts:  strings.Join(compressed.Concepts, " "),
					Files:     strings.Join(compressed.Files, " "),
					Type:      compressed.Type,
				}
				if err := w.indexer.Index(doc); err != nil {
					log.Printf("[pipeline] Worker %d index error: %v", id, err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
