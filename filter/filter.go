package filter

import (
	"context"
	"strings"
	"sync"

	"github.com/s0up4200/arrbiter/radarr"
)

// Default instances with lazy initialization
var (
	defaultCompiler  Compiler
	defaultEvaluator *ConcurrentEvaluator
	initOnce         sync.Once
)

// initDefaults initializes the default instances
func initDefaults() {
	initOnce.Do(func() {
		// Create compiler with caching enabled
		defaultCompiler = NewExprCompiler(WithCache(100))
		// Create evaluator with default settings
		defaultEvaluator = NewConcurrentEvaluator()
	})
}

// ParseAndCreateFilter parses a filter expression and returns a filter function
func ParseAndCreateFilter(expression string) (func(radarr.MovieInfo) bool, error) {
	initDefaults()

	expression = strings.TrimSpace(expression)
	if expression == "" {
		// Empty filter matches everything
		return func(radarr.MovieInfo) bool { return true }, nil
	}

	// Compile the filter
	compiled, err := defaultCompiler.Compile(expression)
	if err != nil {
		return nil, err
	}

	// Return a function that evaluates the filter
	return func(movie radarr.MovieInfo) bool {
		return compiled.Evaluate(movie)
	}, nil
}

// CompileFilter compiles a filter expression into a CompiledFilter
func CompileFilter(expression string) (CompiledFilter, error) {
	initDefaults()

	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, &CompilationError{
			Expression: expression,
			Reason:     "empty expression",
		}
	}

	return defaultCompiler.Compile(expression)
}

// EvaluateFilters evaluates multiple filters against movies concurrently
func EvaluateFilters(ctx context.Context, filters map[string]string, movies []radarr.MovieInfo) (map[string][]radarr.MovieInfo, error) {
	initDefaults()

	// Compile all filters
	compiled := make(map[string]CompiledFilter, len(filters))
	for name, expr := range filters {
		filter, err := CompileFilter(expr)
		if err != nil {
			return nil, &CompilationError{
				Expression: expr,
				Reason:     "failed to compile filter '" + name + "'",
				Err:        err,
			}
		}
		compiled[name] = filter
	}

	// Evaluate concurrently
	return defaultEvaluator.EvaluateBatch(ctx, compiled, movies)
}

// CreateExprFilter creates a filter function from an expression
func CreateExprFilter(expression string) (func(radarr.MovieInfo) bool, error) {
	return ParseAndCreateFilter(expression)
}
