package filter

import (
	"context"

	"github.com/s0up4200/arrbiter/radarr"
)

// Filter defines the basic interface for movie filters
type Filter interface {
	// Evaluate checks if a movie matches the filter criteria
	Evaluate(movie radarr.MovieInfo) bool
}

// CompiledFilter represents a pre-compiled filter ready for evaluation
type CompiledFilter interface {
	Filter

	// Expression returns the original filter expression
	Expression() string

	// IsThreadSafe indicates if the filter can be evaluated concurrently
	IsThreadSafe() bool
}

// Compiler compiles filter expressions into executable filters
type Compiler interface {
	// Compile parses and compiles a filter expression
	Compile(expression string) (CompiledFilter, error)
}

// Evaluator evaluates filters against movies
type Evaluator interface {
	// Evaluate evaluates a filter against all movies
	Evaluate(ctx context.Context, filter CompiledFilter, movies []radarr.MovieInfo) ([]radarr.MovieInfo, error)
}

// BatchEvaluator evaluates multiple filters concurrently
type BatchEvaluator interface {
	// EvaluateBatch evaluates multiple filters against movies concurrently
	EvaluateBatch(ctx context.Context, filters map[string]CompiledFilter, movies []radarr.MovieInfo) (map[string][]radarr.MovieInfo, error)
}

// CachingCompiler provides caching for compiled filters
type CachingCompiler interface {
	Compiler

	// Clear removes all cached filters
	Clear()

	// Size returns the number of cached filters
	Size() int
}

// BatchResult represents the result of evaluating a filter
type BatchResult struct {
	FilterName string
	Matches    []radarr.MovieInfo
	Error      error
}

// WorkerPool defines the interface for concurrent work execution
type WorkerPool interface {
	// Submit submits work to the pool
	Submit(work func()) error

	// Stop gracefully stops the worker pool
	Stop(ctx context.Context) error
}
