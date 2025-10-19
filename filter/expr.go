package filter

import (
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/s0up4200/arrbiter/radarr"
)

// exprFilter implements CompiledFilter using the expr language
type exprFilter struct {
	expression string
	program    *vm.Program
}

// ExprCompilerOption configures an expr compiler
type ExprCompilerOption func(*exprCompiler)

// WithCache enables filter caching with the specified size
func WithCache(size int) ExprCompilerOption {
	return func(c *exprCompiler) {
		if size > 0 {
			c.cache = newLRUCache(size)
		}
	}
}

// WithCustomFunctions adds custom helper functions
func WithCustomFunctions(funcs map[string]any) ExprCompilerOption {
	return func(c *exprCompiler) {
		maps.Copy(c.helperFuncs, funcs)
	}
}

// NewExprCompiler creates a new expr-based filter compiler
func NewExprCompiler(opts ...ExprCompilerOption) Compiler {
	c := &exprCompiler{
		helperFuncs: createHelperFunctions(),
		envPool:     &sync.Pool{},
	}

	for _, opt := range opts {
		opt(c)
	}

	// Initialize environment pool
	c.envPool.New = func() any {
		return make(map[string]any, 64) // Pre-size for typical use
	}

	return c
}

// exprCompiler implements Compiler for expr-based filters
type exprCompiler struct {
	helperFuncs map[string]any
	cache       *lruCache
	envPool     *sync.Pool // Pool for environment maps
}

// Compile compiles an expression into an executable filter
func (c *exprCompiler) Compile(expression string) (CompiledFilter, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, &CompilationError{
			Expression: expression,
			Reason:     "empty expression",
		}
	}

	// Check cache if enabled
	if c.cache != nil {
		if cached, ok := c.cache.Get(expression); ok {
			return cached.(CompiledFilter), nil
		}
	}

	// Compile with static environment for validation
	program, err := expr.Compile(expression,
		expr.Env(c.helperFuncs),
		expr.AllowUndefinedVariables(), // Allow movie properties
		expr.AsBool(),                  // Ensure boolean result
	)
	if err != nil {
		return nil, &CompilationError{
			Expression: expression,
			Reason:     "failed to compile expression",
			Err:        err,
		}
	}

	filter := &exprFilter{
		expression: expression,
		program:    program,
	}

	// Cache if enabled
	if c.cache != nil {
		c.cache.Put(expression, filter)
	}

	return filter, nil
}

// Clear removes all cached filters
func (c *exprCompiler) Clear() {
	if c.cache != nil {
		c.cache.Clear()
	}
}

// Size returns the number of cached filters
func (c *exprCompiler) Size() int {
	if c.cache != nil {
		return c.cache.Size()
	}
	return 0
}

// Evaluate evaluates the filter against a movie
func (f *exprFilter) Evaluate(movie radarr.MovieInfo) bool {
	// Create runtime environment with movie data and dynamic helpers
	env := createRuntimeEnvironment(movie)

	result, err := expr.Run(f.program, env)
	if err != nil {
		// In production, we might want to log this error
		// For now, we return false to skip movies that cause errors
		return false
	}

	// Result is guaranteed to be bool due to AsBool() option during compilation
	return result.(bool)
}

// Expression returns the original expression
func (f *exprFilter) Expression() string {
	return f.expression
}

// IsThreadSafe indicates that expr filters are thread-safe
func (f *exprFilter) IsThreadSafe() bool {
	return true
}

// createHelperFunctions creates the static helper functions used during compilation
func createHelperFunctions() map[string]any {
	funcs := make(map[string]any, 32)
	addHelperFunctions(funcs)
	return funcs
}

// addHelperFunctions adds all helper functions to the provided map
func addHelperFunctions(env map[string]any) {
	// Date helpers
	env["daysSince"] = func(t time.Time) int {
		return int(time.Since(t).Hours() / 24)
	}
	env["daysAgo"] = func(days int) time.Time {
		return time.Now().AddDate(0, 0, -days)
	}
	env["monthsAgo"] = func(months int) time.Time {
		return time.Now().AddDate(0, -months, 0)
	}
	env["yearsAgo"] = func(years int) time.Time {
		return time.Now().AddDate(-years, 0, 0)
	}
	env["parseDate"] = func(dateStr string) time.Time {
		t, _ := time.Parse("2006-01-02", dateStr)
		return t
	}
	// String helpers
	env["contains"] = func(str, substr string) bool {
		return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
	}
	env["startsWith"] = func(str, prefix string) bool {
		return strings.HasPrefix(strings.ToLower(str), strings.ToLower(prefix))
	}
	env["endsWith"] = func(str, suffix string) bool {
		return strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix))
	}
	env["lower"] = strings.ToLower
	env["upper"] = strings.ToUpper
	// Current time
	env["now"] = time.Now
}

// createRuntimeEnvironment creates the runtime environment for filter evaluation
func createRuntimeEnvironment(movie radarr.MovieInfo) map[string]any {
	// Pre-allocate with expected size
	env := make(map[string]any, 64)

	// Add helper functions
	addHelperFunctions(env)

	// Add movie data
	env["Movie"] = movie

	// Add movie-specific helper functions using closures for efficiency
	env["hasTag"] = createHasTagFunc(movie.TagNames)
	env["watchedBy"] = createWatchedByFunc(movie.UserWatchData)
	env["watchCountBy"] = createWatchCountByFunc(movie.UserWatchData)
	env["watchProgressBy"] = createWatchProgressByFunc(movie.UserWatchData)

	// Rating helpers using closures
	ratings := movie.Ratings
	env["imdbRating"] = createRatingFunc(ratings, "imdb")
	env["tmdbRating"] = createRatingFunc(ratings, "tmdb")
	env["rottenTomatoesRating"] = createRatingFunc(ratings, "rottenTomatoes")
	env["metacriticRating"] = createRatingFunc(ratings, "metacritic")
	env["hasRating"] = createHasRatingFunc(ratings)
	env["getRating"] = createGetRatingFunc(ratings)

	// Request helpers using closures
	env["requestedBy"] = createRequestedByFunc(movie.IsRequested, movie.RequestedBy)
	env["requestedAfter"] = createRequestedAfterFunc(movie.IsRequested, movie.RequestDate)
	env["requestedBefore"] = createRequestedBeforeFunc(movie.IsRequested, movie.RequestDate)
	env["requestStatus"] = createRequestStatusFunc(movie.IsRequested, movie.RequestStatus)
	env["approvedBy"] = createApprovedByFunc(movie.IsRequested, movie.ApprovedBy)
	env["isRequested"] = createIsRequestedFunc(movie.IsRequested)
	env["notRequested"] = createNotRequestedFunc(movie.IsRequested)
	env["notWatchedByRequester"] = createNotWatchedByRequesterFunc(movie.IsRequested, movie.RequestedBy, movie.UserWatchData)
	env["watchedByRequester"] = createWatchedByRequesterFunc(movie.IsRequested, movie.RequestedBy, movie.UserWatchData)

	// Direct movie properties for convenience
	env["Title"] = movie.Title
	env["Year"] = movie.Year
	env["Tags"] = movie.TagNames
	env["Added"] = movie.Added
	env["FileImported"] = movie.FileImported
	env["Watched"] = movie.Watched
	env["WatchCount"] = movie.WatchCount
	env["LastWatched"] = movie.LastWatched
	env["WatchProgress"] = movie.WatchProgress
	env["HasFile"] = movie.HasFile
	env["Path"] = movie.Path
	env["IMDBID"] = movie.IMDBID
	env["TMDBID"] = movie.TMDBID
	env["Ratings"] = movie.Ratings
	env["Popularity"] = movie.Popularity
	// Request properties
	env["RequestedBy"] = movie.RequestedBy
	env["RequestedByEmail"] = movie.RequestedByEmail
	env["RequestDate"] = movie.RequestDate
	env["RequestStatus"] = movie.RequestStatus
	env["ApprovedBy"] = movie.ApprovedBy
	env["IsAutoRequest"] = movie.IsAutoRequest
	env["IsRequested"] = movie.IsRequested

	return env
}

// Helper factory functions for better performance through closures

func createHasTagFunc(tags []string) func(string) bool {
	// Pre-convert to lowercase for case-insensitive comparison
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}
	return func(tag string) bool {
		target := strings.ToLower(tag)
		return slices.Contains(lowerTags, target)
	}
}

func createWatchedByFunc(watchData map[string]*radarr.UserWatchInfo) func(string) bool {
	return func(username string) bool {
		if userData, exists := watchData[username]; exists {
			return userData.Watched
		}
		return false
	}
}

func createWatchCountByFunc(watchData map[string]*radarr.UserWatchInfo) func(string) int {
	return func(username string) int {
		if userData, exists := watchData[username]; exists {
			return userData.WatchCount
		}
		return 0
	}
}

func createWatchProgressByFunc(watchData map[string]*radarr.UserWatchInfo) func(string) float64 {
	return func(username string) float64 {
		if userData, exists := watchData[username]; exists {
			return userData.MaxProgress
		}
		return 0
	}
}

func createRatingFunc(ratings map[string]float64, source string) func() float64 {
	rating, ok := ratings[source]
	return func() float64 {
		if ok {
			return rating
		}
		return 0
	}
}

func createHasRatingFunc(ratings map[string]float64) func(string) bool {
	return func(source string) bool {
		_, exists := ratings[source]
		return exists
	}
}

func createGetRatingFunc(ratings map[string]float64) func(string) float64 {
	return func(source string) float64 {
		if val, ok := ratings[source]; ok {
			return val
		}
		return 0
	}
}

func createRequestedByFunc(isRequested bool, requestedBy string) func(string) bool {
	return func(username string) bool {
		return isRequested && strings.EqualFold(requestedBy, username)
	}
}

func createRequestedAfterFunc(isRequested bool, requestDate time.Time) func(time.Time) bool {
	return func(date time.Time) bool {
		return isRequested && requestDate.After(date)
	}
}

func createRequestedBeforeFunc(isRequested bool, requestDate time.Time) func(time.Time) bool {
	return func(date time.Time) bool {
		return isRequested && requestDate.Before(date)
	}
}

func createRequestStatusFunc(isRequested bool, requestStatus string) func(string) bool {
	lowerStatus := strings.ToLower(requestStatus)
	return func(status string) bool {
		return isRequested && strings.ToLower(status) == lowerStatus
	}
}

func createApprovedByFunc(isRequested bool, approvedBy string) func(string) bool {
	return func(username string) bool {
		return isRequested && strings.EqualFold(approvedBy, username)
	}
}

func createIsRequestedFunc(isRequested bool) func() bool {
	return func() bool {
		return isRequested
	}
}

func createNotRequestedFunc(isRequested bool) func() bool {
	return func() bool {
		return !isRequested
	}
}

func createNotWatchedByRequesterFunc(isRequested bool, requestedBy string, watchData map[string]*radarr.UserWatchInfo) func() bool {
	return func() bool {
		if !isRequested || requestedBy == "" {
			return false
		}
		// Check if the requester has watched it
		const minWatchPercent = 85.0 // Default watch threshold
		if userData, exists := watchData[requestedBy]; exists {
			return userData.MaxProgress < minWatchPercent
		}
		return true // Not watched if no watch data
	}
}

func createWatchedByRequesterFunc(isRequested bool, requestedBy string, watchData map[string]*radarr.UserWatchInfo) func() bool {
	return func() bool {
		if !isRequested || requestedBy == "" {
			return false
		}
		// Check if the requester has watched it
		const minWatchPercent = 85.0 // Default watch threshold
		if userData, exists := watchData[requestedBy]; exists {
			return userData.MaxProgress >= minWatchPercent
		}
		return false // Not watched if no watch data
	}
}
