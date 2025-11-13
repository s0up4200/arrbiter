package filter

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/s0up4200/arrbiter/radarr"
)

// Manager provides advanced filter management capabilities
type Manager struct {
	compiler  Compiler
	evaluator *ConcurrentEvaluator
	filters   map[string]CompiledFilter
	mu        sync.RWMutex
}

// ManagerOption configures a filter manager
type ManagerOption func(*Manager)

// WithCompiler sets a custom compiler
func WithCompiler(compiler Compiler) ManagerOption {
	return func(m *Manager) {
		m.compiler = compiler
	}
}

// WithEvaluator sets a custom evaluator
func WithEvaluator(evaluator *ConcurrentEvaluator) ManagerOption {
	return func(m *Manager) {
		m.evaluator = evaluator
	}
}

// NewManager creates a new filter manager
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		compiler:  NewExprCompiler(WithCache(100)),
		evaluator: NewConcurrentEvaluator(),
		filters:   make(map[string]CompiledFilter),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// RegisterFilter registers a new filter or updates an existing one
func (m *Manager) RegisterFilter(name, expression string) error {
	filter, err := m.compiler.Compile(expression)
	if err != nil {
		return fmt.Errorf("failed to compile filter '%s': %w", name, err)
	}

	m.mu.Lock()
	m.filters[name] = filter
	m.mu.Unlock()

	return nil
}

// RegisterFilters registers multiple filters at once
func (m *Manager) RegisterFilters(filters map[string]string) error {
	compiled := make(map[string]CompiledFilter, len(filters))

	// Compile all filters first
	for name, expr := range filters {
		filter, err := m.compiler.Compile(expr)
		if err != nil {
			return fmt.Errorf("failed to compile filter '%s': %w", name, err)
		}
		compiled[name] = filter
	}

	// If all compiled successfully, register them
	m.mu.Lock()
	maps.Copy(m.filters, compiled)
	m.mu.Unlock()

	return nil
}

// UnregisterFilter removes a filter
func (m *Manager) UnregisterFilter(name string) {
	m.mu.Lock()
	delete(m.filters, name)
	m.mu.Unlock()
}

// GetFilter returns a compiled filter by name
func (m *Manager) GetFilter(name string) (CompiledFilter, bool) {
	m.mu.RLock()
	filter, exists := m.filters[name]
	m.mu.RUnlock()
	return filter, exists
}

// ListFilters returns all registered filter names
func (m *Manager) ListFilters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.filters))
	for name := range m.filters {
		names = append(names, name)
	}
	return names
}

// EvaluateFilter evaluates a single registered filter
func (m *Manager) EvaluateFilter(ctx context.Context, name string, movies []radarr.MovieInfo) ([]radarr.MovieInfo, error) {
	filter, exists := m.GetFilter(name)
	if !exists {
		return nil, fmt.Errorf("filter '%s' not found", name)
	}

	return m.evaluator.Evaluate(ctx, filter, movies)
}

// EvaluateAll evaluates all registered filters
func (m *Manager) EvaluateAll(ctx context.Context, movies []radarr.MovieInfo) (map[string][]radarr.MovieInfo, error) {
	m.mu.RLock()
	filters := make(map[string]CompiledFilter, len(m.filters))
	maps.Copy(filters, m.filters)
	m.mu.RUnlock()

	return m.evaluator.EvaluateBatch(ctx, filters, movies)
}

// EvaluateSelected evaluates only the specified filters
func (m *Manager) EvaluateSelected(ctx context.Context, filterNames []string, movies []radarr.MovieInfo) (map[string][]radarr.MovieInfo, error) {
	m.mu.RLock()
	filters := make(map[string]CompiledFilter, len(filterNames))
	for _, name := range filterNames {
		if filter, exists := m.filters[name]; exists {
			filters[name] = filter
		} else {
			m.mu.RUnlock()
			return nil, fmt.Errorf("filter '%s' not found", name)
		}
	}
	m.mu.RUnlock()

	return m.evaluator.EvaluateBatch(ctx, filters, movies)
}

// Close gracefully shuts down the manager
func (m *Manager) Close(ctx context.Context) error {
	return m.evaluator.Stop(ctx)
}
