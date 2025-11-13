package radarr

import (
	"context"
	"fmt"
	"time"

	"golift.io/starr/radarr"
)

// UpgradeOptions contains options for upgrade operations
type UpgradeOptions struct {
	TargetCustomFormats []string // Names of custom formats to look for
	MinFormatScore      int      // Minimum custom format score required
	CheckAvailability   bool     // Whether to check if movie is released before searching
	DryRun              bool     // Whether to run in dry-run mode
}

// UpgradeResult contains information about a movie that needs upgrading
type UpgradeResult struct {
	Movie               MovieInfo
	CurrentFormats      []string // Current custom formats
	CurrentFormatScore  int
	MissingFormats      []string // Target formats that are missing
	IsAvailable         bool     // Whether the movie is released
	NeedsMonitoring     bool     // Whether monitoring needs to be enabled
}

// ScanMoviesForUpgrade finds movies missing the configured custom formats
func (o *Operations) ScanMoviesForUpgrade(ctx context.Context, opts UpgradeOptions) ([]UpgradeResult, error) {
	o.logger.Info().
		Strs("target_formats", opts.TargetCustomFormats).
		Int("min_score", opts.MinFormatScore).
		Msg("Scanning movies for upgrade opportunities")

	// Get all movies from Radarr
	movies, err := o.client.GetAllMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}

	// Get tags for mapping
	tags, err := o.client.GetTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	// Get all custom formats for name lookups
	customFormats, err := o.client.GetCustomFormats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom formats: %w", err)
	}

	// Create a map of custom format ID to name
	formatNameMap := make(map[int64]string)
	for _, cf := range customFormats {
		formatNameMap[cf.ID] = cf.Name
	}

	// Process movie files concurrently to get detailed custom format data
	if err := o.client.ProcessMovieFiles(ctx, movies); err != nil {
		o.logger.Warn().Err(err).Msg("Failed to process some movie files")
	}

	var results []UpgradeResult
	var processedCount int

	for _, movie := range movies {
		// Skip movies without files
		if movie.MovieFile == nil || movie.MovieFile.Path == "" {
			continue
		}

		processedCount++

		// Convert to MovieInfo
		info := o.client.GetMovieInfo(movie, tags)

		// Check if movie is available if requested
		isAvailable := true
		if opts.CheckAvailability {
			isAvailable = o.IsMovieAvailable(movie)
		}

		// Get current custom formats from the movie file
		var currentFormats []string
		currentScore := movie.MovieFile.CustomFormatScore
		
		// Use the custom formats from the movie file (already processed concurrently)
		if movie.MovieFile != nil && movie.MovieFile.CustomFormats != nil {
			for _, cf := range movie.MovieFile.CustomFormats {
				if cf != nil && cf.Name != "" {
					currentFormats = append(currentFormats, cf.Name)
				}
			}
		}

		// Check which target formats are missing
		var missingFormats []string
		currentFormatsMap := make(map[string]bool)
		for _, format := range currentFormats {
			currentFormatsMap[format] = true
		}

		for _, targetFormat := range opts.TargetCustomFormats {
			if !currentFormatsMap[targetFormat] {
				missingFormats = append(missingFormats, targetFormat)
			}
		}

		// Determine if this movie needs upgrading
		needsUpgrade := false
		if len(missingFormats) > 0 {
			needsUpgrade = true
		}
		if opts.MinFormatScore > 0 && currentScore < opts.MinFormatScore {
			needsUpgrade = true
		}

		if needsUpgrade {
			result := UpgradeResult{
				Movie:              info,
				CurrentFormats:     currentFormats,
				CurrentFormatScore: currentScore,
				MissingFormats:     missingFormats,
				IsAvailable:        isAvailable,
				NeedsMonitoring:    !movie.Monitored,
			}
			results = append(results, result)
		}
	}

	o.logger.Info().
		Int("total_processed", processedCount).
		Int("upgrade_candidates", len(results)).
		Msg("Completed upgrade scan")

	return results, nil
}

// IsMovieAvailable checks if a movie is released (checks physical and digital release dates)
func (o *Operations) IsMovieAvailable(movie *radarr.Movie) bool {
	// If the movie is marked as available by Radarr, trust that
	if movie.IsAvailable {
		return true
	}

	now := time.Now()

	// Check digital release date
	if !movie.DigitalRelease.IsZero() && movie.DigitalRelease.Before(now) {
		return true
	}

	// Check physical release date
	if !movie.PhysicalRelease.IsZero() && movie.PhysicalRelease.Before(now) {
		return true
	}

	// Check in cinemas date (with some buffer time for home release)
	if !movie.InCinemas.IsZero() {
		// Typically movies are available for home release 3-4 months after cinema release
		homeReleaseBuffer := movie.InCinemas.AddDate(0, 4, 0)
		if homeReleaseBuffer.Before(now) {
			return true
		}
	}

	return false
}

// MonitorMovie enables monitoring on a movie
func (o *Operations) MonitorMovie(ctx context.Context, movieID int64) error {
	o.logger.Info().Int64("movie_id", movieID).Msg("Enabling monitoring for movie")

	// Get the current movie data
	movie, err := o.client.GetMovieByID(ctx, movieID)
	if err != nil {
		return fmt.Errorf("failed to get movie: %w", err)
	}

	// If already monitored, nothing to do
	if movie.Monitored {
		o.logger.Debug().Int64("movie_id", movieID).Msg("Movie already monitored")
		return nil
	}

	// Enable monitoring
	movie.Monitored = true
	_, err = o.client.UpdateMovie(ctx, movie)
	if err != nil {
		return fmt.Errorf("failed to update movie monitoring: %w", err)
	}

	o.logger.Info().
		Int64("movie_id", movieID).
		Str("title", movie.Title).
		Msg("Successfully enabled monitoring")

	return nil
}

// TriggerUpgradeSearch searches for better quality versions of movies
func (o *Operations) TriggerUpgradeSearch(ctx context.Context, movieIDs []int64) error {
	if len(movieIDs) == 0 {
		return fmt.Errorf("no movie IDs provided for upgrade search")
	}

	o.logger.Info().
		Ints64("movie_ids", movieIDs).
		Msg("Triggering upgrade search for movies")

	// Use the MoviesSearch command to search for better versions
	searchCommand := &radarr.CommandRequest{
		Name:     "MoviesSearch",
		MovieIDs: movieIDs,
	}

	response, err := o.client.SendCommand(ctx, searchCommand)
	if err != nil {
		return fmt.Errorf("failed to trigger movie search: %w", err)
	}

	o.logger.Info().
		Int64("command_id", response.ID).
		Str("status", response.Status).
		Msg("Successfully triggered upgrade search")

	return nil
}

// ProcessUpgrades handles the upgrade workflow for a list of upgrade candidates
func (o *Operations) ProcessUpgrades(ctx context.Context, candidates []UpgradeResult, opts UpgradeOptions) error {
	if len(candidates) == 0 {
		o.logger.Info().Msg("No movies need upgrading")
		return nil
	}

	if opts.DryRun {
		o.logger.Info().Msg("DRY RUN MODE - No changes will be made")
		fmt.Print(o.formatter.FormatUpgradeCandidates(candidates))
		return nil
	}

	// Group by actions needed
	var toMonitor []int64
	var toSearch []int64

	for _, candidate := range candidates {
		// Enable monitoring if needed
		if candidate.NeedsMonitoring {
			toMonitor = append(toMonitor, candidate.Movie.ID)
		}

		// Only search if available
		if candidate.IsAvailable {
			toSearch = append(toSearch, candidate.Movie.ID)
		}
	}

	// Enable monitoring for unmonitored movies
	if len(toMonitor) > 0 {
		o.logger.Info().Int("count", len(toMonitor)).Msg("Enabling monitoring for movies")
		for _, movieID := range toMonitor {
			if err := o.MonitorMovie(ctx, movieID); err != nil {
				o.logger.Error().Err(err).Int64("movie_id", movieID).Msg("Failed to enable monitoring")
				// Continue with other movies
			}
		}
	}

	// Trigger searches using concurrent batch processing
	if len(toSearch) > 0 {
		o.logger.Info().Int("count", len(toSearch)).Msg("Triggering upgrade searches")
		if err := o.client.BatchSearchMovies(ctx, toSearch); err != nil {
			o.logger.Error().Err(err).Msg("Failed to trigger some searches")
		}
	}

	o.logger.Info().
		Int("monitored", len(toMonitor)).
		Int("searched", len(toSearch)).
		Msg("Upgrade processing complete")

	return nil
}

