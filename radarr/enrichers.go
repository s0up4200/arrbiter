package radarr

import (
	"context"
	"fmt"

	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/tautulli"
)

// tautulliEnricher implements MovieEnricher for Tautulli integration
type tautulliEnricher struct {
	operations *Operations
}

// EnrichMovies adds watch status information from Tautulli
func (e *tautulliEnricher) EnrichMovies(ctx context.Context, movies []MovieInfo) error {
	if e.operations.tautulliClient == nil {
		return nil
	}

	// Create identifiers for batch lookup
	var identifiers []tautulli.MovieIdentifier
	for _, movie := range movies {
		identifiers = append(identifiers, tautulli.MovieIdentifier{
			IMDbID: movie.IMDBID,
			TMDbID: movie.TMDBID,
			Title:  movie.Title,
		})
	}

	// Get watch status with per-user data for all movies at once
	watchStatuses, err := e.operations.tautulliClient.BatchGetMovieWatchStatusWithUsers(
		ctx, identifiers, e.operations.minWatchPercent)
	if err != nil {
		return fmt.Errorf("failed to fetch watch status: %w", err)
	}

	// Update movie info with watch status
	for i := range movies {
		if status, ok := watchStatuses[movies[i].IMDBID]; ok {
			movies[i].Watched = status.Watched
			movies[i].WatchCount = status.WatchCount
			movies[i].LastWatched = status.LastWatched
			movies[i].WatchProgress = status.MaxProgress

			// Initialize map if nil
			if movies[i].UserWatchData == nil {
				movies[i].UserWatchData = make(map[string]*UserWatchInfo)
			}

			// Copy per-user data
			for username, userData := range status.UserData {
				movies[i].UserWatchData[username] = &UserWatchInfo{
					Username:    userData.Username,
					Watched:     userData.Watched,
					WatchCount:  userData.WatchCount,
					LastWatched: userData.LastWatched,
					MaxProgress: userData.MaxProgress,
				}
			}
		}
	}

	return nil
}

// overseerrEnricher implements MovieEnricher for Overseerr integration
type overseerrEnricher struct {
	operations *Operations
}

// EnrichMovies adds request information from Overseerr
func (e *overseerrEnricher) EnrichMovies(ctx context.Context, movies []MovieInfo) error {
	if e.operations.overseerrClient == nil {
		return nil
	}

	// Get all movie requests from Overseerr
	requests, err := e.operations.overseerrClient.GetMovieRequests(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch movie requests: %w", err)
	}

	// Create a map of TMDB ID to requests for efficient lookup
	requestsByTMDB := make(map[int64][]overseerr.MediaRequest)
	for _, req := range requests {
		tmdbID := int64(req.Media.TmdbID)
		requestsByTMDB[tmdbID] = append(requestsByTMDB[tmdbID], req)
	}

	// Update movie info with request data
	for i := range movies {
		if requests, ok := requestsByTMDB[movies[i].TMDBID]; ok && len(requests) > 0 {
			// Use the most recent request
			var latestRequest overseerr.MediaRequest
			for _, req := range requests {
				if latestRequest.ID == 0 || req.CreatedAt.After(latestRequest.CreatedAt) {
					latestRequest = req
				}
			}

			// Convert and assign request data
			requestData := latestRequest.ToMovieRequest()
			movies[i].RequestedBy = requestData.RequestedBy
			movies[i].RequestedByEmail = requestData.RequestedByEmail
			movies[i].RequestDate = requestData.RequestDate
			movies[i].RequestStatus = requestData.RequestStatus
			movies[i].ApprovedBy = requestData.ApprovedBy
			movies[i].IsAutoRequest = requestData.IsAutoRequest
			movies[i].IsRequested = true
		}
	}

	e.operations.logger.Debug().
		Int("total_requests", len(requests)).
		Int("matched_movies", len(requestsByTMDB)).
		Msg("Enriched movies with Overseerr request data")

	return nil
}

// addEnricher adds an enricher to the operations if not already present
func (o *Operations) addEnricher(enricher MovieEnricher) {
	// Check if enricher type already exists
	for _, existing := range o.enrichers {
		if fmt.Sprintf("%T", existing) == fmt.Sprintf("%T", enricher) {
			return
		}
	}
	o.enrichers = append(o.enrichers, enricher)
}