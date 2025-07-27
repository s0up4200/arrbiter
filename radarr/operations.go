package radarr

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"golift.io/starr/radarr"

	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/qbittorrent"
	"github.com/s0up4200/arrbiter/tautulli"
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
	client            *Client
	tautulliClient    *tautulli.Client
	overseerrClient   *overseerr.Client
	qbittorrentClient *qbittorrent.Client
	logger            zerolog.Logger
	minWatchPercent   float64
	formatter         MovieFormatter
	enrichers         []MovieEnricher
}

// NewOperations creates a new Operations instance
func NewOperations(client *Client, logger zerolog.Logger) *Operations {
	return &Operations{
		client:          client,
		logger:          logger,
		minWatchPercent: 85.0, // Default value
		formatter:       NewConsoleFormatter(),
		enrichers:       make([]MovieEnricher, 0),
	}
}

// SetTautulliClient sets the Tautulli client for watch status lookups
func (o *Operations) SetTautulliClient(client *tautulli.Client) {
	o.tautulliClient = client
	// Add to enrichers if not already present
	o.addEnricher(&tautulliEnricher{operations: o})
}

// SetMinWatchPercent sets the minimum watch percentage for considering a movie watched
func (o *Operations) SetMinWatchPercent(percent float64) {
	o.minWatchPercent = percent
}

// SetOverseerrClient sets the Overseerr client for request data lookups
func (o *Operations) SetOverseerrClient(client *overseerr.Client) {
	o.overseerrClient = client
	// Add to enrichers if not already present
	o.addEnricher(&overseerrEnricher{operations: o})
}

// GetAllMovies returns all movies with enriched data
func (o *Operations) GetAllMovies(ctx context.Context) ([]MovieInfo, error) {
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

	// Convert to MovieInfo with tags
	var results []MovieInfo
	for _, movie := range movies {
		info := o.client.GetMovieInfo(movie, tags)
		// Only include movies with imported files
		if !info.FileImported.IsZero() {
			results = append(results, info)
		}
	}

	// Enrich movies from all configured sources concurrently
	if len(o.enrichers) > 0 {
		if err := o.client.EnrichMoviesFromMultipleSources(ctx, results, o.enrichers...); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to enrich movies from all sources")
		}
	}

	return results, nil
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

	// Enrich movies from all configured sources concurrently
	if len(o.enrichers) > 0 {
		if err := o.client.EnrichMoviesFromMultipleSources(ctx, movieInfos, o.enrichers...); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to enrich movies from all sources")
		}
	}

	// Filter movies - only consider movies with imported files
	var results []MovieInfo
	for _, info := range movieInfos {
		// Skip movies without imported files
		if info.FileImported.IsZero() {
			continue
		}
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


// DeleteMovies deletes movies matching the filter
func (o *Operations) DeleteMovies(ctx context.Context, movies []MovieInfo, opts DeleteOptions) error {
	if len(movies) == 0 {
		o.logger.Info().Msg("No movies to delete")
		return nil
	}

	if opts.DryRun {
		o.logger.Info().Msg("DRY RUN MODE - No movies will be deleted")
		fmt.Print(o.formatter.FormatMoviesToDelete(movies))
		return nil
	}

	if opts.ConfirmDelete {
		fmt.Print(o.formatter.FormatMoviesToDelete(movies))
		if !o.confirmDeletion(len(movies)) {
			o.logger.Info().Msg("Deletion cancelled by user")
			return nil
		}
	}

	// Use concurrent batch deletion
	result := o.client.BatchDeleteMovies(ctx, movies, opts.DeleteFiles)

	o.logger.Info().
		Int("deleted", len(result.Successful)).
		Int("failed", len(result.Failed)).
		Msg("Deletion complete")

	// Log individual failures
	for _, failure := range result.Failed {
		o.logger.Error().
			Err(failure.Err).
			Int64("id", failure.MovieID).
			Str("title", failure.MovieTitle).
			Msg("Failed to delete movie")
	}

	if len(result.Failed) > 0 {
		return fmt.Errorf("failed to delete %d movies", len(result.Failed))
	}

	return nil
}


// confirmDeletion prompts the user for confirmation
func (o *Operations) confirmDeletion(count int) bool {
	fmt.Printf("\nAre you sure you want to delete %d movie(s)? [y/N]: ", count)

	var response string
	fmt.Scanln(&response)

	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

// ImportOptions contains options for manual import operations
type ImportOptions struct {
	Path         string
	MovieID      int64
	ImportMode   string // "move" or "copy"
	Quality      string
	IncludeExtra bool
}

// ScanForImports scans a folder for importable movie files
func (o *Operations) ScanForImports(ctx context.Context, opts ImportOptions) ([]*radarr.ManualImportOutput, error) {
	params := &radarr.ManualImportParams{
		Folder:              opts.Path,
		FilterExistingFiles: true,
	}

	// If specific movie ID provided, filter to that movie
	if opts.MovieID > 0 {
		params.MovieID = opts.MovieID
	}

	o.logger.Info().Str("path", opts.Path).Msg("Scanning folder for importable movies")

	output, err := o.client.GetManualImportItems(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to scan for imports: %w", err)
	}

	if len(output) == 0 {
		o.logger.Info().Msg("No importable files found")
		return nil, nil
	}

	o.logger.Info().Msgf("Found %d importable files", len(output))
	return output, nil
}

// ImportMovies processes manual import of selected movie files
func (o *Operations) ImportMovies(ctx context.Context, items []*radarr.ManualImportInput, opts ImportOptions) error {
	if len(items) == 0 {
		o.logger.Info().Msg("No items to import")
		return nil
	}

	o.logger.Info().Msgf("Importing %d files", len(items))

	// Process imports
	if err := o.client.ProcessManualImport(ctx, items); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	o.logger.Info().Msg("Import completed successfully")
	return nil
}

// PrintImportableItems prints importable items in a formatted way
func (o *Operations) PrintImportableItems(items []*radarr.ManualImportOutput) {
	if o.formatter != nil {
		o.formatter.(*ConsoleFormatter).PrintImportableItems(items)
	}
}

// ConvertToImportInput converts ManualImportOutput items to ManualImportInput for processing
func (o *Operations) ConvertToImportInput(outputs []*radarr.ManualImportOutput, importMode string) []*radarr.ManualImportInput {
	var inputs []*radarr.ManualImportInput

	for _, output := range outputs {
		// Skip items with rejections
		if len(output.Rejections) > 0 {
			continue
		}

		// Skip if no movie is associated
		if output.Movie == nil {
			continue
		}

		input := &radarr.ManualImportInput{
			ID:                output.ID,
			Path:              output.Path,
			MovieID:           output.Movie.ID,
			Movie:             output.Movie,
			Quality:           output.Quality,
			Languages:         output.Languages,
			ReleaseGroup:      output.ReleaseGroup,
			DownloadID:        output.DownloadID,
			CustomFormats:     convertCustomFormats(output.CustomFormats),
			CustomFormatScore: output.CustomFormatScore,
		}

		inputs = append(inputs, input)
	}

	return inputs
}

// convertCustomFormats converts CustomFormatOutput to CustomFormatInput
func convertCustomFormats(outputs []*radarr.CustomFormatOutput) []*radarr.CustomFormatInput {
	if outputs == nil {
		return nil
	}

	var inputs []*radarr.CustomFormatInput
	for _, output := range outputs {
		input := &radarr.CustomFormatInput{
			ID:   output.ID,
			Name: output.Name,
		}
		inputs = append(inputs, input)
	}
	return inputs
}

