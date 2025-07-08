package radarr

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/soup/radarr-cleanup/tautulli"
)

// SearchOptions contains options for searching movies
type SearchOptions struct {
	FilterExpression string
	ShowDetails      bool
}

// DeleteOptions contains options for deleting movies
type DeleteOptions struct {
	DryRun        bool
	DeleteFiles   bool
	ConfirmDelete bool
}

// Operations handles movie search and delete operations
type Operations struct {
	client          *Client
	tautulliClient  *tautulli.Client
	logger          zerolog.Logger
	minWatchPercent float64
}

// NewOperations creates a new Operations instance
func NewOperations(client *Client, logger zerolog.Logger) *Operations {
	return &Operations{
		client:          client,
		logger:          logger,
		minWatchPercent: 85.0, // Default value
	}
}

// SetTautulliClient sets the Tautulli client for watch status lookups
func (o *Operations) SetTautulliClient(client *tautulli.Client) {
	o.tautulliClient = client
}

// SetMinWatchPercent sets the minimum watch percentage for considering a movie watched
func (o *Operations) SetMinWatchPercent(percent float64) {
	o.minWatchPercent = percent
}

// SearchMovies searches for movies matching the filter expression
func (o *Operations) SearchMovies(ctx context.Context, filterFunc func(MovieInfo) bool) ([]MovieInfo, error) {
	// Get all movies
	movies, err := o.client.GetAllMovies(ctx)
	if err != nil {
		return nil, err
	}
	
	// Get all tags for mapping
	tags, err := o.client.GetTags(ctx)
	if err != nil {
		return nil, err
	}
	
	// Convert movies to MovieInfo
	var movieInfos []MovieInfo
	for _, movie := range movies {
		info := o.client.GetMovieInfo(movie, tags)
		movieInfos = append(movieInfos, info)
	}
	
	// Fetch watch status if Tautulli is configured
	if o.tautulliClient != nil {
		o.logger.Debug().Msg("Fetching watch status from Tautulli")
		if err := o.enrichWithWatchStatus(ctx, movieInfos); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to fetch watch status from Tautulli")
			// Continue without watch status
		}
	}
	
	// Filter movies
	var results []MovieInfo
	for _, info := range movieInfos {
		if filterFunc(info) {
			results = append(results, info)
		}
	}
	
	// Sort by title
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
	})
	
	o.logger.Info().Msgf("Found %d movies matching filter", len(results))
	return results, nil
}

// enrichWithWatchStatus adds watch status information to movies
func (o *Operations) enrichWithWatchStatus(ctx context.Context, movies []MovieInfo) error {
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
	watchStatuses, err := o.tautulliClient.BatchGetMovieWatchStatusWithUsers(ctx, identifiers, o.minWatchPercent)
	if err != nil {
		return err
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

// DeleteMovies deletes movies matching the filter
func (o *Operations) DeleteMovies(ctx context.Context, movies []MovieInfo, opts DeleteOptions) error {
	if len(movies) == 0 {
		o.logger.Info().Msg("No movies to delete")
		return nil
	}
	
	if opts.DryRun {
		o.logger.Info().Msg("DRY RUN MODE - No movies will be deleted")
		o.printMoviesToDelete(movies)
		return nil
	}
	
	if opts.ConfirmDelete {
		o.printMoviesToDelete(movies)
		if !o.confirmDeletion(len(movies)) {
			o.logger.Info().Msg("Deletion cancelled by user")
			return nil
		}
	}
	
	// Delete movies
	var deletedCount int
	var errors []error
	
	for _, movie := range movies {
		o.logger.Info().
			Int64("id", movie.ID).
			Str("title", movie.Title).
			Int("year", movie.Year).
			Msg("Deleting movie")
		
		if err := o.client.DeleteMovie(ctx, movie.ID, opts.DeleteFiles); err != nil {
			o.logger.Error().Err(err).
				Int64("id", movie.ID).
				Str("title", movie.Title).
				Msg("Failed to delete movie")
			errors = append(errors, err)
		} else {
			deletedCount++
		}
	}
	
	o.logger.Info().
		Int("deleted", deletedCount).
		Int("failed", len(errors)).
		Msg("Deletion complete")
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to delete %d movies", len(errors))
	}
	
	return nil
}

// printMoviesToDelete prints the list of movies to be deleted
func (o *Operations) printMoviesToDelete(movies []MovieInfo) {
	fmt.Printf("\nMovies to be deleted (%d):\n", len(movies))
	fmt.Println(strings.Repeat("-", 80))
	
	var watchedCount int
	for _, movie := range movies {
		fmt.Printf("• %s (%d)", movie.Title, movie.Year)
		
		// Show watch status if available
		if movie.Watched {
			fmt.Printf(" [WATCHED]")
			watchedCount++
		}
		fmt.Println()
		
		if len(movie.TagNames) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(movie.TagNames, ", "))
		}
		if movie.HasFile {
			fmt.Printf("  File: %s\n", movie.Path)
		}
		fmt.Printf("  Added: %s\n", movie.Added.Format("2006-01-02"))
		if !movie.FileImported.IsZero() {
			fmt.Printf("  Imported: %s\n", movie.FileImported.Format("2006-01-02"))
		}
		if movie.WatchCount > 0 {
			fmt.Printf("  Watch Count: %d", movie.WatchCount)
			if !movie.LastWatched.IsZero() {
				fmt.Printf(" (Last: %s)", movie.LastWatched.Format("2006-01-02"))
			}
			fmt.Println()
		}
		fmt.Println()
	}
	fmt.Println(strings.Repeat("-", 80))
	
	if watchedCount > 0 {
		fmt.Printf("\n⚠️  WARNING: %d of %d movies have been watched!\n", watchedCount, len(movies))
		fmt.Println("Use --ignore-watched flag to bypass this warning.")
	}
}

// confirmDeletion prompts the user for confirmation
func (o *Operations) confirmDeletion(count int) bool {
	fmt.Printf("\nAre you sure you want to delete %d movie(s)? [y/N]: ", count)
	
	var response string
	fmt.Scanln(&response)
	
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}