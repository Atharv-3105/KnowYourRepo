package worker

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestPool_ProcessesEnqueuedJobs(t *testing.T) {
	var processed atomic.Int32
	done := make(chan struct{}, 3)

	p := NewPool(2, 8, func(ctx context.Context, jobID string) error {
		processed.Add(1)
		done <- struct{}{}
		return nil
	}, quietLogger())
	p.Start(context.Background())

	for _, id := range []string{"a", "b", "c"} {
		if !p.Enqueue(id) {
			t.Fatalf("enqueue %s rejected", id)
		}
	}
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for jobs")
		}
	}

	if processed.Load() != 3 {
		t.Errorf("expected 3 processed jobs, got %d", processed.Load())
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned %v", err)
	}
}

// The constructor previously took (bufferSize, size) and was called as
// (workerCount, queueSize) - silently producing 16 workers and a queue of
// 2. Named fields make the two impossible to swap.
func TestPool_WorkerCountIsHonored(t *testing.T) {
	var concurrent, peak atomic.Int32
	release := make(chan struct{})

	p := NewPool(2, 8, func(ctx context.Context, jobID string) error {
		n := concurrent.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		<-release
		concurrent.Add(-1)
		return nil
	}, quietLogger())
	p.Start(context.Background())

	for i := 0; i < 6; i++ {
		p.Enqueue(string(rune('a' + i)))
	}
	time.Sleep(100 * time.Millisecond)
	close(release)

	if got := peak.Load(); got != 2 {
		t.Errorf("expected at most 2 concurrent workers, observed %d", got)
	}
	p.Shutdown(context.Background())
}

// Graceful shutdown: an in-flight job that doesn't finish before the
// deadline has its context cancelled rather than being killed mid-write
// with no chance to record its state.
func TestPool_ShutdownCancelsInFlightJobAfterDeadline(t *testing.T) {
	cancelled := make(chan struct{})
	started := make(chan struct{})

	p := NewPool(1, 4, func(ctx context.Context, jobID string) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}, quietLogger())
	p.Start(context.Background())
	p.Enqueue("slow")
	<-started

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	p.Shutdown(shutdownCtx)

	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the in-flight job's context to be cancelled")
	}
}

func TestPool_EnqueueAfterShutdownIsRejectedNotPanics(t *testing.T) {
	p := NewPool(1, 4, func(ctx context.Context, jobID string) error { return nil }, quietLogger())
	p.Start(context.Background())
	p.Shutdown(context.Background())

	if p.Enqueue("late") {
		t.Error("expected Enqueue after Shutdown to be rejected")
	}
}

func TestPool_EnqueueRejectsWhenQueueFull(t *testing.T) {
	block := make(chan struct{})
	p := NewPool(1, 1, func(ctx context.Context, jobID string) error { <-block; return nil }, quietLogger())
	p.Start(context.Background())

	p.Enqueue("running") // picked up by the single worker
	time.Sleep(20 * time.Millisecond)
	if !p.Enqueue("queued") {
		t.Fatal("expected one job to fit in the queue")
	}
	if p.Enqueue("overflow") {
		t.Error("expected enqueue to be rejected once the queue is full")
	}
	close(block)
	p.Shutdown(context.Background())
}
