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

		// Get current custom formats (we already checked MovieFile is not nil above)
		var currentFormats []string
		currentScore := movie.MovieFile.CustomFormatScore
		for _, cf := range movie.MovieFile.CustomFormats {
			if name, ok := formatNameMap[cf.ID]; ok {
				currentFormats = append(currentFormats, name)
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
		o.printUpgradeCandidates(candidates)
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

	// Trigger searches
	if len(toSearch) > 0 {
		o.logger.Info().Int("count", len(toSearch)).Msg("Triggering upgrade searches")
		// Process in batches of 10 to avoid overwhelming the system
		batchSize := 10
		for i := 0; i < len(toSearch); i += batchSize {
			end := i + batchSize
			if end > len(toSearch) {
				end = len(toSearch)
			}
			
			batch := toSearch[i:end]
			if err := o.TriggerUpgradeSearch(ctx, batch); err != nil {
				o.logger.Error().Err(err).Msg("Failed to trigger search batch")
				// Continue with other batches
			}
			
			// Add a small delay between batches
			if end < len(toSearch) {
				time.Sleep(2 * time.Second)
			}
		}
	}

	o.logger.Info().
		Int("monitored", len(toMonitor)).
		Int("searched", len(toSearch)).
		Msg("Upgrade processing complete")

	return nil
}

// printUpgradeCandidates displays upgrade candidates for user review
func (o *Operations) printUpgradeCandidates(candidates []UpgradeResult) {
	fmt.Printf("\nMovies that can be upgraded (%d):\n\n", len(candidates))

	for i, candidate := range candidates {
		isLast := i == len(candidates)-1
		prefix := "\u251c"
		if isLast {
			prefix = "\u2570"
		}

		fmt.Printf("%s\u2500\u2500 %s (%d)\n", prefix, candidate.Movie.Title, candidate.Movie.Year)

		indent := "\u2502   "
		if isLast {
			indent = "    "
		}

		// Current formats and score
		if len(candidate.CurrentFormats) > 0 {
			fmt.Printf("%sCurrent Formats: %v (Score: %d)\n", indent, candidate.CurrentFormats, candidate.CurrentFormatScore)
		} else {
			fmt.Printf("%sCurrent Formats: None (Score: %d)\n", indent, candidate.CurrentFormatScore)
		}

		// Missing formats
		if len(candidate.MissingFormats) > 0 {
			fmt.Printf("%sMissing Formats: %v\n", indent, candidate.MissingFormats)
		}

		// Status info
		statusParts := []string{}
		if !candidate.IsAvailable {
			statusParts = append(statusParts, "Not Released")
		}
		if candidate.NeedsMonitoring {
			statusParts = append(statusParts, "Not Monitored")
		}
		if len(statusParts) > 0 {
			fmt.Printf("%sStatus: %s\n", indent, fmt.Sprint(statusParts))
		}

		// File info
		if candidate.Movie.MovieFile != nil && candidate.Movie.MovieFile.Path != "" {
			fmt.Printf("%sFile: %s\n", indent, candidate.Movie.MovieFile.Path)
		}

		if i < len(candidates)-1 {
			fmt.Printf("\u2502\n")
		}
	}
	fmt.Println()
}

// GetMovieByID retrieves a single movie by its ID
func (c *Client) GetMovieByID(ctx context.Context, movieID int64) (*radarr.Movie, error) {
	movie, err := c.client.GetMovieByIDContext(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie ID %d: %w", movieID, err)
	}
	return movie, nil
}