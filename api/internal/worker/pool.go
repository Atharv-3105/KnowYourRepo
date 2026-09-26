// Package worker runs ingestion jobs in the background so POST /repos can
// return 202 immediately instead of holding the HTTP request open for the
// whole clone/parse/embed pipeline.
package worker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/atharva-3105/KnowYourRepo/internal/metrics"
)

// Handler processes a single job identified by its ID.
type Handler func(ctx context.Context, jobID string) error

// Pool is a fixed-size worker pool fed by a bounded queue. It deliberately
// has no persistence of its own - the ingestion_jobs table is the durable
// record, and interrupted jobs are re-enqueued from it at startup.
type Pool struct {
	jobs    chan string
	handler Handler
	logger  *slog.Logger
	workers int

	mu     sync.Mutex
	closed bool

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewPool creates a pool with `workers` concurrent workers and room for
// `queueSize` waiting jobs. (The previous signature was (bufferSize, size)
// and its one caller passed (workerCount, queueSize) - swapped - so
// production silently ran 16 concurrent ingestions with a queue of 2.
// Parameter names now say what they are.)
func NewPool(workers, queueSize int, handler Handler, logger *slog.Logger) *Pool {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}
	return &Pool{
		jobs:    make(chan string, queueSize),
		handler: handler,
		logger:  logger,
		workers: workers,
	}
}

// Start launches the workers. They run until Shutdown.
func (p *Pool) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}

	p.logger.Info("worker_pool_started", "workers", p.workers, "queue_size", cap(p.jobs))
}

func (p *Pool) runWorker(ctx context.Context, workerID int) {
	defer p.wg.Done()

	for jobID := range p.jobs {
		metrics.WorkerPoolQueueDepth.Set(float64(len(p.jobs)))
		p.logger.Info("worker_job_started", "worker_id", workerID, "job_id", jobID)

		if err := p.handler(ctx, jobID); err != nil {
			p.logger.Error("worker_job_failed", "worker_id", workerID, "job_id", jobID, "error", err)
			continue
		}

		p.logger.Info("worker_job_completed", "worker_id", workerID, "job_id", jobID)
	}

	p.logger.Info("worker_stopped", "worker_id", workerID)
}

// Enqueue submits a job without blocking. It returns false if the queue is
// full or the pool is shutting down, so the caller can answer 503 instead
// of hanging the request.
func (p *Pool) Enqueue(jobID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return false
	}

	select {
	case p.jobs <- jobID:
		metrics.WorkerPoolQueueDepth.Set(float64(len(p.jobs)))
		return true
	default:
		return false
	}
}

// Shutdown stops accepting jobs and waits for workers to drain the queue.
// If ctx expires first, in-flight jobs have their context cancelled so they
// can record an "interrupted" state instead of being killed mid-write, and
// Shutdown waits for them to return. Queued-but-unstarted jobs are left
// in the database as pending and resumed on the next start.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.jobs)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if p.cancel != nil {
			p.cancel()
		}
		return nil
	case <-ctx.Done():
		if p.cancel != nil {
			p.cancel()
		}
		<-done
		return ctx.Err()
	}
}
