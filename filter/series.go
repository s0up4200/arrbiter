package filter

import (
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/s0up4200/arrbiter/sonarr"
)

// lookupUser finds per-user data by username, case-insensitively.
func lookupUser[T any](m map[string]T, username string) (T, bool) {
	if v, ok := m[username]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, username) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// ParseAndCreateSeriesFilter parses a filter expression and returns a series filter function.
func ParseAndCreateSeriesFilter(expression string) (func(sonarr.SeriesInfo) bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return func(sonarr.SeriesInfo) bool { return true }, nil
	}

	program, err := expr.Compile(expression,
		expr.Env(createSeriesRuntimeEnvironment(sonarr.SeriesInfo{})),
		expr.AsBool(),
	)
	if err != nil {
		return nil, &CompilationError{
			Expression: expression,
			Reason:     "failed to compile series filter expression",
			Err:        err,
		}
	}

	return func(series sonarr.SeriesInfo) bool {
		return evaluateSeriesFilter(program, series)
	}, nil
}

func evaluateSeriesFilter(program *vm.Program, series sonarr.SeriesInfo) bool {
	env := createSeriesRuntimeEnvironment(series)
	result, err := expr.Run(program, env)
	if err != nil {
		return false
	}
	return result.(bool)
}

// createSeriesRuntimeEnvironment creates the runtime environment for series filter evaluation.
func createSeriesRuntimeEnvironment(series sonarr.SeriesInfo) map[string]any {
	env := make(map[string]any, 64)

	addHelperFunctions(env)

	env["Series"] = series

	env["hasTag"] = createHasTagFunc(series.TagNames)

	watchData := series.UserWatchData
	env["watchedBy"] = func(username string) bool {
		if userData, exists := lookupUser(watchData, username); exists {
			return userData.Watched
		}
		return false
	}
	env["watchCountBy"] = func(username string) int {
		if userData, exists := lookupUser(watchData, username); exists {
			return userData.WatchCount
		}
		return 0
	}
	env["watchProgressBy"] = func(username string) float64 {
		if userData, exists := lookupUser(watchData, username); exists {
			return userData.Progress
		}
		return 0
	}
	env["watchedEpisodesBy"] = func(username string) int {
		if userData, exists := lookupUser(watchData, username); exists {
			return userData.WatchedEpisodes
		}
		return 0
	}
	env["lastWatchedBy"] = func(username string) time.Time {
		if userData, exists := lookupUser(watchData, username); exists {
			return userData.LastWatched
		}
		return time.Time{}
	}

	env["requestedBy"] = createRequestedByFunc(series.IsRequested, series.RequestedBy)
	env["requestedAfter"] = createRequestedAfterFunc(series.IsRequested, series.RequestDate)
	env["requestedBefore"] = createRequestedBeforeFunc(series.IsRequested, series.RequestDate)
	env["requestStatus"] = createRequestStatusFunc(series.IsRequested, series.RequestStatus)
	env["approvedBy"] = createApprovedByFunc(series.IsRequested, series.ApprovedBy)
	env["isRequested"] = createIsRequestedFunc(series.IsRequested)
	env["notRequested"] = createNotRequestedFunc(series.IsRequested)
	env["watchedByRequester"] = func() bool {
		if !series.IsRequested || series.RequestedBy == "" {
			return false
		}
		if userData, exists := lookupUser(watchData, series.RequestedBy); exists {
			return userData.Watched
		}
		return false
	}
	env["notWatchedByRequester"] = func() bool {
		if !series.IsRequested || series.RequestedBy == "" {
			return false
		}
		if userData, exists := lookupUser(watchData, series.RequestedBy); exists {
			return !userData.Watched
		}
		return true
	}

	env["Title"] = series.Title
	env["Year"] = series.Year
	env["Tags"] = series.TagNames
	env["Added"] = series.Added
	env["Ended"] = series.Ended
	env["Status"] = series.Status
	env["EpisodeCount"] = series.EpisodeCount
	env["TotalEpisodes"] = series.TotalEpisodes
	env["SizeOnDisk"] = series.SizeOnDisk
	env["Path"] = series.Path
	env["TvdbID"] = series.TvdbID
	env["IMDBID"] = series.IMDBID
	env["Watched"] = series.Watched
	env["WatchedEpisodes"] = series.WatchedEpisodes
	env["WatchCount"] = series.WatchCount
	env["LastWatched"] = series.LastWatched
	env["WatchProgress"] = series.WatchProgress
	env["RequestedBy"] = series.RequestedBy
	env["RequestedByEmail"] = series.RequestedByEmail
	env["RequestDate"] = series.RequestDate
	env["RequestStatus"] = series.RequestStatus
	env["ApprovedBy"] = series.ApprovedBy
	env["IsAutoRequest"] = series.IsAutoRequest
	env["IsRequested"] = series.IsRequested

	return env
}
