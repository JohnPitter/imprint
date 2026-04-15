package pipeline

import (
	"context"
	"log"
	"sync"

	"imprint/internal/store"
)

// Job represents a compression job for a raw observation.
type Job struct {
	Raw *store.RawObservationRow
}

// Worker is a background worker pool for processing LLM pipeline jobs.
type Worker struct {
	compressor *Compressor
	obsStore   *store.ObservationStore
	jobs       chan Job
	wg         sync.WaitGroup
	cancel     context.CancelFunc
}

// NewWorker creates a new worker pool with numWorkers goroutines.
func NewWorker(compressor *Compressor, obsStore *store.ObservationStore, numWorkers int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		compressor: compressor,
		obsStore:   obsStore,
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
			}
		case <-ctx.Done():
			return
		}
	}
}
