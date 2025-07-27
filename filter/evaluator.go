package filter

import (
	"context"
	"runtime"
	"sync"

	"github.com/s0up4200/arrbiter/radarr"
)

// EvaluatorOption configures an evaluator
type EvaluatorOption func(*ConcurrentEvaluator)

// WithWorkers sets the number of worker goroutines
func WithWorkers(workers int) EvaluatorOption {
	return func(e *ConcurrentEvaluator) {
		e.workerCount = workers
	}
}

// WithBatchSize sets the batch size for chunked processing
func WithBatchSize(size int) EvaluatorOption {
	return func(e *ConcurrentEvaluator) {
		e.batchSize = size
	}
}

// ConcurrentEvaluator implements both Evaluator and BatchEvaluator interfaces
type ConcurrentEvaluator struct {
	workerCount int
	batchSize   int
	pool        WorkerPool
}

// NewConcurrentEvaluator creates a new concurrent evaluator
func NewConcurrentEvaluator(opts ...EvaluatorOption) *ConcurrentEvaluator {
	e := &ConcurrentEvaluator{
		workerCount: runtime.GOMAXPROCS(0),
		batchSize:   100,
	}

	for _, opt := range opts {
		opt(e)
	}

	e.pool = NewWorkerPool(e.workerCount)

	return e
}

// Evaluate evaluates a single filter against all movies
func (e *ConcurrentEvaluator) Evaluate(ctx context.Context, filter CompiledFilter, movies []radarr.MovieInfo) ([]radarr.MovieInfo, error) {
	if len(movies) == 0 {
		return []radarr.MovieInfo{}, nil
	}

	// For small movie lists, don't bother with concurrency
	if len(movies) < e.batchSize {
		return e.evaluateSequential(filter, movies), nil
	}

	// Use concurrent evaluation for large movie lists
	return e.evaluateConcurrent(ctx, filter, movies)
}

// EvaluateBatch evaluates multiple filters against movies concurrently
func (e *ConcurrentEvaluator) EvaluateBatch(ctx context.Context, filters map[string]CompiledFilter, movies []radarr.MovieInfo) (map[string][]radarr.MovieInfo, error) {
	if len(filters) == 0 || len(movies) == 0 {
		return make(map[string][]radarr.MovieInfo), nil
	}

	results := make(map[string][]radarr.MovieInfo)
	resultChan := make(chan BatchResult, len(filters))

	var wg sync.WaitGroup
	for name, filter := range filters {
		wg.Add(1)
		name := name // Capture loop variable
		filter := filter

		err := e.pool.Submit(func() {
			defer wg.Done()

			select {
			case <-ctx.Done():
				resultChan <- BatchResult{
					FilterName: name,
					Error:      ctx.Err(),
				}
				return
			default:
			}

			matches, err := e.Evaluate(ctx, filter, movies)
			resultChan <- BatchResult{
				FilterName: name,
				Matches:    matches,
				Error:      err,
			}
		})

		if err != nil {
			wg.Done()
			// Pool is stopped, return early
			return nil, err
		}
	}

	// Close result channel when all work is done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		if result.Error != nil {
			// Skip filters that error
			continue
		}
		results[result.FilterName] = result.Matches
	}

	return results, nil
}

// evaluateSequential evaluates a filter against all movies sequentially
func (e *ConcurrentEvaluator) evaluateSequential(filter CompiledFilter, movies []radarr.MovieInfo) []radarr.MovieInfo {
	matches := make([]radarr.MovieInfo, 0, len(movies)/10) // Pre-allocate with estimate
	for _, movie := range movies {
		if filter.Evaluate(movie) {
			matches = append(matches, movie)
		}
	}
	return matches
}

// evaluateConcurrent evaluates a filter against movies using the worker pool
func (e *ConcurrentEvaluator) evaluateConcurrent(ctx context.Context, filter CompiledFilter, movies []radarr.MovieInfo) ([]radarr.MovieInfo, error) {
	// Calculate chunk size
	chunkSize := max(len(movies)/e.workerCount, e.batchSize)

	// Result collection
	type chunkResult struct {
		matches []radarr.MovieInfo
		order   int
	}

	resultChan := make(chan chunkResult, (len(movies)/chunkSize)+1)
	var wg sync.WaitGroup

	// Process chunks concurrently
	chunkIndex := 0
	for i := 0; i < len(movies); i += chunkSize {
		end := min(i+chunkSize, len(movies))

		wg.Add(1)
		chunk := movies[i:end]
		index := chunkIndex
		chunkIndex++

		err := e.pool.Submit(func() {
			defer wg.Done()

			// Check context
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Evaluate chunk
			matches := make([]radarr.MovieInfo, 0, len(chunk)/10)
			for _, movie := range chunk {
				if filter.Evaluate(movie) {
					matches = append(matches, movie)
				}
			}

			resultChan <- chunkResult{matches: matches, order: index}
		})

		if err != nil {
			wg.Done()
			return nil, err
		}
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results maintaining order
	results := make(map[int][]radarr.MovieInfo)
	for result := range resultChan {
		results[result.order] = result.matches
	}

	// Check context one more time
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Combine results in order
	totalMatches := 0
	for i := 0; i < len(results); i++ {
		totalMatches += len(results[i])
	}

	allMatches := make([]radarr.MovieInfo, 0, totalMatches)
	for i := 0; i < len(results); i++ {
		allMatches = append(allMatches, results[i]...)
	}

	return allMatches, nil
}

// Stop gracefully stops the evaluator's worker pool
func (e *ConcurrentEvaluator) Stop(ctx context.Context) error {
	return e.pool.Stop(ctx)
}
