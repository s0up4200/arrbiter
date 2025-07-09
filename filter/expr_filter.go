package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/soup/radarr-cleanup/radarr"
)

// ExprFilter represents a compiled expr filter
type ExprFilter struct {
	program *vm.Program
	expr    string
}

// CompileExprFilter compiles an expr filter expression
func CompileExprFilter(expression string) (*ExprFilter, error) {
	if strings.TrimSpace(expression) == "" {
		return nil, fmt.Errorf("empty filter expression")
	}
	
	// Define static helper functions that can be used in expressions
	env := map[string]interface{}{
		// Date helpers
		"daysSince": func(t time.Time) int {
			return int(time.Since(t).Hours() / 24)
		},
		"daysAgo": func(days int) time.Time {
			return time.Now().AddDate(0, 0, -days)
		},
		"monthsAgo": func(months int) time.Time {
			return time.Now().AddDate(0, -months, 0)
		},
		"yearsAgo": func(years int) time.Time {
			return time.Now().AddDate(-years, 0, 0)
		},
		"parseDate": func(dateStr string) time.Time {
			t, _ := time.Parse("2006-01-02", dateStr)
			return t
		},
		// String helpers
		"contains": func(str, substr string) bool {
			return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
		},
		"startsWith": func(str, prefix string) bool {
			return strings.HasPrefix(strings.ToLower(str), strings.ToLower(prefix))
		},
		"endsWith": func(str, suffix string) bool {
			return strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix))
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		// Current time
		"now": time.Now,
	}
	
	// Compile the expression
	program, err := expr.Compile(expression, 
		expr.Env(env),
		expr.AllowUndefinedVariables(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compile filter expression: %w", err)
	}
	
	return &ExprFilter{
		program: program,
		expr:    expression,
	}, nil
}

// Evaluate evaluates the filter against a movie
func (f *ExprFilter) Evaluate(movie radarr.MovieInfo) bool {
	// Create environment with movie data and helper functions
	env := map[string]interface{}{
		// Movie data
		"Movie": movie,
		
		// Tag helpers
		"hasTag": func(tag string) bool {
			for _, t := range movie.TagNames {
				if strings.EqualFold(t, tag) {
					return true
				}
			}
			return false
		},
		
		// User watch helpers
		"watchedBy": func(username string) bool {
			if userData, exists := movie.UserWatchData[username]; exists {
				return userData.Watched
			}
			return false
		},
		
		"watchCountBy": func(username string) int {
			if userData, exists := movie.UserWatchData[username]; exists {
				return userData.WatchCount
			}
			return 0
		},
		
		"watchProgressBy": func(username string) float64 {
			if userData, exists := movie.UserWatchData[username]; exists {
				return userData.MaxProgress
			}
			return 0
		},
		
		// Date helpers
		"daysSince": func(t time.Time) int {
			return int(time.Since(t).Hours() / 24)
		},
		"daysAgo": func(days int) time.Time {
			return time.Now().AddDate(0, 0, -days)
		},
		"monthsAgo": func(months int) time.Time {
			return time.Now().AddDate(0, -months, 0)
		},
		"yearsAgo": func(years int) time.Time {
			return time.Now().AddDate(-years, 0, 0)
		},
		"parseDate": func(dateStr string) time.Time {
			t, _ := time.Parse("2006-01-02", dateStr)
			return t
		},
		
		// String helpers
		"contains": func(str, substr string) bool {
			return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
		},
		"startsWith": func(str, prefix string) bool {
			return strings.HasPrefix(strings.ToLower(str), strings.ToLower(prefix))
		},
		"endsWith": func(str, suffix string) bool {
			return strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix))
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		
		// Current time
		"now": time.Now,
		
		// Rating helpers
		"imdbRating": func() float64 {
			if val, ok := movie.Ratings["imdb"]; ok {
				return val
			}
			return 0
		},
		"tmdbRating": func() float64 {
			if val, ok := movie.Ratings["tmdb"]; ok {
				return val
			}
			return 0
		},
		"rottenTomatoesRating": func() float64 {
			if val, ok := movie.Ratings["rottenTomatoes"]; ok {
				return val
			}
			return 0
		},
		"metacriticRating": func() float64 {
			if val, ok := movie.Ratings["metacritic"]; ok {
				return val
			}
			return 0
		},
		"hasRating": func(source string) bool {
			_, exists := movie.Ratings[source]
			return exists
		},
		"getRating": func(source string) float64 {
			if val, ok := movie.Ratings[source]; ok {
				return val
			}
			return 0
		},
		
		// Request helpers (Overseerr integration)
		"requestedBy": func(username string) bool {
			return movie.IsRequested && strings.EqualFold(movie.RequestedBy, username)
		},
		"requestedAfter": func(date time.Time) bool {
			return movie.IsRequested && movie.RequestDate.After(date)
		},
		"requestedBefore": func(date time.Time) bool {
			return movie.IsRequested && movie.RequestDate.Before(date)
		},
		"requestStatus": func(status string) bool {
			return movie.IsRequested && strings.EqualFold(movie.RequestStatus, status)
		},
		"approvedBy": func(username string) bool {
			return movie.IsRequested && strings.EqualFold(movie.ApprovedBy, username)
		},
		"isRequested": func() bool {
			return movie.IsRequested
		},
		"notRequested": func() bool {
			return !movie.IsRequested
		},
		"notWatchedByRequester": func() bool {
			if !movie.IsRequested || movie.RequestedBy == "" {
				return false
			}
			// Check if the requester has watched it
			minWatchPercent := 85.0 // Default watch threshold
			if userData, exists := movie.UserWatchData[movie.RequestedBy]; exists {
				return userData.MaxProgress < minWatchPercent
			}
			return true // Not watched if no watch data
		},
		"watchedByRequester": func() bool {
			if !movie.IsRequested || movie.RequestedBy == "" {
				return false
			}
			// Check if the requester has watched it
			minWatchPercent := 85.0 // Default watch threshold
			if userData, exists := movie.UserWatchData[movie.RequestedBy]; exists {
				return userData.MaxProgress >= minWatchPercent
			}
			return false // Not watched if no watch data
		},
		
		// Direct movie properties for convenience
		"Title":         movie.Title,
		"Year":          movie.Year,
		"Tags":          movie.TagNames,
		"Added":         movie.Added,
		"FileImported":  movie.FileImported,
		"Watched":       movie.Watched,
		"WatchCount":    movie.WatchCount,
		"LastWatched":   movie.LastWatched,
		"WatchProgress": movie.WatchProgress,
		"HasFile":       movie.HasFile,
		"Path":          movie.Path,
		"IMDBID":        movie.IMDBID,
		"TMDBID":        movie.TMDBID,
		"Ratings":       movie.Ratings,
		"Popularity":    movie.Popularity,
		// Request properties
		"RequestedBy":      movie.RequestedBy,
		"RequestedByEmail": movie.RequestedByEmail,
		"RequestDate":      movie.RequestDate,
		"RequestStatus":    movie.RequestStatus,
		"ApprovedBy":       movie.ApprovedBy,
		"IsAutoRequest":    movie.IsAutoRequest,
		"IsRequested":      movie.IsRequested,
	}
	
	result, err := expr.Run(f.program, env)
	if err != nil {
		// Log error but return false to skip the movie
		return false
	}
	
	// Convert result to boolean
	if boolResult, ok := result.(bool); ok {
		return boolResult
	}
	
	return false
}

// String returns the original expression
func (f *ExprFilter) String() string {
	return f.expr
}

// CreateExprFilter creates a filter function from an expression
func CreateExprFilter(expression string) (func(radarr.MovieInfo) bool, error) {
	filter, err := CompileExprFilter(expression)
	if err != nil {
		return nil, err
	}
	
	return func(movie radarr.MovieInfo) bool {
		return filter.Evaluate(movie)
	}, nil
}