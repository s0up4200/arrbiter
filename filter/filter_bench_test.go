package filter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/s0up4200/arrbiter/radarr"
)

// generateTestMovies creates test movie data
func generateTestMovies(count int) []radarr.MovieInfo {
	movies := make([]radarr.MovieInfo, count)

	for i := 0; i < count; i++ {
		movies[i] = radarr.MovieInfo{
			ID:         int64(i),
			Title:      fmt.Sprintf("Movie %d", i),
			Year:       2020 + (i % 5),
			Added:      time.Now().AddDate(0, -i%12, 0),
			TagNames:   []string{"action", "drama", "sci-fi"}[:(i%3)+1],
			Watched:    i%2 == 0,
			WatchCount: i % 5,
			HasFile:    true,
			Path:       fmt.Sprintf("/movies/movie%d", i),
			Ratings: map[string]float64{
				"imdb": 5.0 + float64(i%5),
				"tmdb": 6.0 + float64(i%4),
			},
			UserWatchData: map[string]*radarr.UserWatchInfo{
				"user1": {
					Watched:     i%3 == 0,
					WatchCount:  i % 3,
					MaxProgress: float64(i % 101),
				},
				"user2": {
					Watched:     i%4 == 0,
					WatchCount:  i % 2,
					MaxProgress: float64((i * 2) % 101),
				},
			},
		}
	}

	return movies
}

// Benchmark filter compilation
func BenchmarkCompileFilter(b *testing.B) {
	expressions := []struct {
		name string
		expr string
	}{
		{"simple", `hasTag("action")`},
		{"complex", `hasTag("action") and Year > 2022 and imdbRating() > 7.0`},
	}

	for _, tc := range expressions {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := CompileFilter(tc.expr)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark filter compilation with caching
func BenchmarkCompileFilterWithCache(b *testing.B) {
	compiler := NewExprCompiler(WithCache(100))
	expression := `hasTag("action") and Year > 2022`

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := compiler.Compile(expression)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark single filter evaluation
func BenchmarkEvaluateFilter(b *testing.B) {
	movies := generateTestMovies(1000)
	filter, _ := CompileFilter(`hasTag("action") and Year > 2021`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		matches := 0
		for _, movie := range movies {
			if filter.Evaluate(movie) {
				matches++
			}
		}
		_ = matches
	}
}

// Benchmark concurrent evaluation
func BenchmarkEvaluateConcurrent(b *testing.B) {
	movies := generateTestMovies(10000)
	filter, _ := CompileFilter(`hasTag("action") and imdbRating() > 7.0`)
	ctx := context.Background()

	evaluators := []struct {
		name      string
		evaluator *ConcurrentEvaluator
	}{
		{"workers-1", NewConcurrentEvaluator(WithWorkers(1))},
		{"workers-4", NewConcurrentEvaluator(WithWorkers(4))},
		{"workers-8", NewConcurrentEvaluator(WithWorkers(8))},
		{"workers-default", NewConcurrentEvaluator()},
	}

	for _, tc := range evaluators {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := tc.evaluator.Evaluate(ctx, filter, movies)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark batch evaluation
func BenchmarkEvaluateBatch(b *testing.B) {
	movies := generateTestMovies(5000)
	filters := map[string]string{
		"action":    `hasTag("action")`,
		"recent":    `Added > monthsAgo(6)`,
		"highRated": `imdbRating() > 8.0`,
		"watched":   `watchedBy("user1")`,
		"complex":   `hasTag("sci-fi") and Year > 2022 and tmdbRating() > 7.5`,
	}

	compiled := make(map[string]CompiledFilter)
	for name, expr := range filters {
		filter, _ := CompileFilter(expr)
		compiled[name] = filter
	}

	ctx := context.Background()
	evaluator := NewConcurrentEvaluator()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := evaluator.EvaluateBatch(ctx, compiled, movies)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark helper function performance
func BenchmarkHelperFunctions(b *testing.B) {
	movie := radarr.MovieInfo{
		TagNames: []string{"action", "drama", "thriller"},
		UserWatchData: map[string]*radarr.UserWatchInfo{
			"user1": {Watched: true, WatchCount: 3},
		},
		Ratings: map[string]float64{
			"imdb": 8.5,
			"tmdb": 8.0,
		},
	}

	b.Run("hasTag", func(b *testing.B) {
		hasTag := createHasTagFunc(movie.TagNames)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = hasTag("action")
		}
	})

	b.Run("watchedBy", func(b *testing.B) {
		watchedBy := createWatchedByFunc(movie.UserWatchData)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = watchedBy("user1")
		}
	})

	b.Run("getRating", func(b *testing.B) {
		getRating := createGetRatingFunc(movie.Ratings)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = getRating("imdb")
		}
	})
}
