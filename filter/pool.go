package filter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrPoolStopped is returned when work is submitted to a stopped pool
var ErrPoolStopped = errors.New("worker pool is stopped")

// workerPool implements WorkerPool with bounded concurrency
type workerPool struct {
	workers  int
	workChan chan func()
	stopOnce sync.Once
	stopped  atomic.Bool
	wg       sync.WaitGroup
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) WorkerPool {
	if workers <= 0 {
		workers = 1
	}

	pool := &workerPool{
		workers:  workers,
		workChan: make(chan func(), workers*2), // Buffer for better throughput
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker processes work from the channel
func (p *workerPool) worker() {
	defer p.wg.Done()

	for work := range p.workChan {
		if work != nil {
			work()
		}
	}
}

// Submit submits work to the pool
func (p *workerPool) Submit(work func()) error {
	if p.stopped.Load() {
		return ErrPoolStopped
	}

	select {
	case p.workChan <- work:
		return nil
	default:
		// Channel is full, block until we can submit
		select {
		case p.workChan <- work:
			return nil
		case <-time.After(time.Second):
			if p.stopped.Load() {
				return ErrPoolStopped
			}
			// Try once more
			p.workChan <- work
			return nil
		}
	}
}

// Stop gracefully stops the worker pool
func (p *workerPool) Stop(ctx context.Context) error {
	var err error

	p.stopOnce.Do(func() {
		p.stopped.Store(true)
		close(p.workChan)

		// Wait for workers with context
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All workers finished
		case <-ctx.Done():
			err = ctx.Err()
		}
	})

	return err
}
