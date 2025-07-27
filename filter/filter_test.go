package filter

import (
	"context"
	"testing"
	"time"

	"github.com/s0up4200/arrbiter/radarr"
)

func TestCompileFilter(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid expression",
			expression: `hasTag("action")`,
			wantErr:    false,
		},
		{
			name:        "empty expression",
			expression:  "",
			wantErr:     true,
			errContains: "empty expression",
		},
		{
			name:       "invalid syntax",
			expression: `hasTag("unclosed`,
			wantErr:    true,
		},
		{
			name:       "complex expression",
			expression: `hasTag("action") and Year > 2020 and imdbRating() > 7.0`,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := CompileFilter(tt.expression)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if filter == nil {
					t.Errorf("expected filter but got nil")
				}
			}
		})
	}
}

func TestFilterEvaluation(t *testing.T) {
	// Create test movie
	movie := radarr.MovieInfo{
		ID:         1,
		Title:      "Test Movie",
		Year:       2023,
		Added:      time.Now().AddDate(-1, 0, 0),
		TagNames:   []string{"action", "sci-fi"},
		Watched:    true,
		WatchCount: 2,
		HasFile:    true,
		Ratings: map[string]float64{
			"imdb": 8.5,
			"tmdb": 8.0,
		},
		UserWatchData: map[string]*radarr.UserWatchInfo{
			"john": {
				Watched:     true,
				WatchCount:  2,
				MaxProgress: 100.0,
			},
			"jane": {
				Watched:     false,
				WatchCount:  0,
				MaxProgress: 45.0,
			},
		},
		IsRequested:   true,
		RequestedBy:   "john",
		RequestDate:   time.Now().AddDate(0, -3, 0),
		RequestStatus: "approved",
	}

	tests := []struct {
		name       string
		expression string
		movie      radarr.MovieInfo
		expected   bool
	}{
		{
			name:       "has tag",
			expression: `hasTag("action")`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "does not have tag",
			expression: `hasTag("horror")`,
			movie:      movie,
			expected:   false,
		},
		{
			name:       "year comparison",
			expression: `Year > 2020`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "rating check",
			expression: `imdbRating() > 8.0`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "watched by user",
			expression: `watchedBy("john")`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "not watched by user",
			expression: `not watchedBy("jane")`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "watch count comparison",
			expression: `watchCountBy("john") >= 2`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "complex expression",
			expression: `hasTag("sci-fi") and imdbRating() > 8.0 and watchedBy("john")`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "request check",
			expression: `requestedBy("john") and requestStatus("approved")`,
			movie:      movie,
			expected:   true,
		},
		{
			name:       "date comparison",
			expression: `Added < daysAgo(30)`,
			movie:      movie,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := CompileFilter(tt.expression)
			if err != nil {
				t.Fatalf("failed to compile filter: %v", err)
			}

			result := filter.Evaluate(tt.movie)
			if result != tt.expected {
				t.Errorf("expected %v but got %v for expression %q", tt.expected, result, tt.expression)
			}
		})
	}
}

func TestConcurrentEvaluation(t *testing.T) {
	// Generate test data
	movies := generateTestMovies(1000)

	filter, err := CompileFilter(`hasTag("action") and Year > 2021`)
	if err != nil {
		t.Fatalf("failed to compile filter: %v", err)
	}

	ctx := context.Background()
	evaluator := NewConcurrentEvaluator(WithWorkers(4))

	matches, err := evaluator.Evaluate(ctx, filter, movies)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Verify results by sequential evaluation
	var expectedMatches []radarr.MovieInfo
	for _, movie := range movies {
		if filter.Evaluate(movie) {
			expectedMatches = append(expectedMatches, movie)
		}
	}

	if len(matches) != len(expectedMatches) {
		t.Errorf("expected %d matches but got %d", len(expectedMatches), len(matches))
	}
}

func TestBatchEvaluation(t *testing.T) {
	movies := generateTestMovies(500)

	filters := map[string]string{
		"action":    `hasTag("action")`,
		"recent":    `Year >= 2023`,
		"highRated": `imdbRating() > 7.0`,
	}

	ctx := context.Background()
	results, err := EvaluateFilters(ctx, filters, movies)
	if err != nil {
		t.Fatalf("batch evaluation failed: %v", err)
	}

	// Verify we got results for all filters
	if len(results) != len(filters) {
		t.Errorf("expected %d filter results but got %d", len(filters), len(results))
	}

	// Verify each filter has reasonable results
	for name, matches := range results {
		if len(matches) == 0 {
			t.Logf("warning: filter %q matched no movies", name)
		}
		t.Logf("filter %q matched %d movies", name, len(matches))
	}
}

func TestFilterManager(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Test registering filters
	filters := map[string]string{
		"action":  `hasTag("action")`,
		"recent":  `Year > 2022`,
		"watched": `Watched == true`,
	}

	err := manager.RegisterFilters(filters)
	if err != nil {
		t.Fatalf("failed to register filters: %v", err)
	}

	// Test listing filters
	names := manager.ListFilters()
	if len(names) != len(filters) {
		t.Errorf("expected %d filters but got %d", len(filters), len(names))
	}

	// Test getting a filter
	filter, exists := manager.GetFilter("action")
	if !exists {
		t.Error("expected filter 'action' to exist")
	}
	if filter == nil {
		t.Error("expected non-nil filter")
	}

	// Test evaluating with manager
	movies := generateTestMovies(100)
	matches, err := manager.EvaluateFilter(ctx, "action", movies)
	if err != nil {
		t.Fatalf("failed to evaluate filter: %v", err)
	}
	if len(matches) == 0 {
		t.Error("expected some matches")
	}

	// Test unregistering
	manager.UnregisterFilter("action")
	_, exists = manager.GetFilter("action")
	if exists {
		t.Error("expected filter 'action' to be removed")
	}
}


func TestCacheEffectiveness(t *testing.T) {
	compiler := NewExprCompiler(WithCache(10))
	expression := `hasTag("action") and Year > 2020`

	// First compilation - should miss cache
	_, err := compiler.Compile(expression)
	if err != nil {
		t.Fatalf("first compilation failed: %v", err)
	}

	// Second compilation - should hit cache
	filter2, err := compiler.Compile(expression)
	if err != nil {
		t.Fatalf("second compilation failed: %v", err)
	}
	if filter2 == nil {
		t.Error("expected non-nil filter from cache")
	}

	// Test cache size
	if cachingCompiler, ok := compiler.(CachingCompiler); ok {
		if cachingCompiler.Size() != 1 {
			t.Errorf("expected cache size 1 but got %d", cachingCompiler.Size())
		}

		// Test clear
		cachingCompiler.Clear()
		if cachingCompiler.Size() != 0 {
			t.Errorf("expected cache size 0 after clear but got %d", cachingCompiler.Size())
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
