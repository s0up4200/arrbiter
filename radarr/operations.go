package radarr

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/qbittorrent"
	"github.com/s0up4200/arrbiter/tautulli"
	"golift.io/starr/radarr"
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

// SetOverseerrClient sets the Overseerr client for request data lookups
func (o *Operations) SetOverseerrClient(client *overseerr.Client) {
	o.overseerrClient = client
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
	
	// Enrich with watch status if Tautulli is configured
	if o.tautulliClient != nil {
		o.logger.Debug().Msg("Fetching watch status from Tautulli")
		if err := o.enrichWithWatchStatus(ctx, results); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to fetch watch status from Tautulli")
		}
	}
	
	// Enrich with request data if Overseerr is configured
	if o.overseerrClient != nil {
		o.logger.Debug().Msg("Fetching request data from Overseerr")
		if err := o.enrichWithRequestData(ctx, results); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to fetch request data from Overseerr")
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
	
	// Fetch watch status if Tautulli is configured
	if o.tautulliClient != nil {
		o.logger.Debug().Msg("Fetching watch status from Tautulli")
		if err := o.enrichWithWatchStatus(ctx, movieInfos); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to fetch watch status from Tautulli")
			// Continue without watch status
		}
	}
	
	// Fetch request data if Overseerr is configured
	if o.overseerrClient != nil {
		o.logger.Debug().Msg("Fetching request data from Overseerr")
		if err := o.enrichWithRequestData(ctx, movieInfos); err != nil {
			o.logger.Warn().Err(err).Msg("Failed to fetch request data from Overseerr")
			// Continue without request data
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

// enrichWithRequestData adds request information from Overseerr to movies
func (o *Operations) enrichWithRequestData(ctx context.Context, movies []MovieInfo) error {
	// Get all movie requests from Overseerr
	requests, err := o.overseerrClient.GetMovieRequests(ctx)
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
			requestData := overseerr.ConvertToMovieRequest(latestRequest)
			movies[i].RequestedBy = requestData.RequestedBy
			movies[i].RequestedByEmail = requestData.RequestedByEmail
			movies[i].RequestDate = requestData.RequestDate
			movies[i].RequestStatus = requestData.RequestStatus
			movies[i].ApprovedBy = requestData.ApprovedBy
			movies[i].IsAutoRequest = requestData.IsAutoRequest
			movies[i].IsRequested = true
		}
	}
	
	o.logger.Debug().
		Int("total_requests", len(requests)).
		Int("matched_movies", len(requestsByTMDB)).
		Msg("Enriched movies with Overseerr request data")
	
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
	fmt.Printf("\nMovie")
	if len(movies) != 1 {
		fmt.Printf("s")
	}
	fmt.Printf(" to be deleted (%d):\n\n", len(movies))
	
	var watchedCount int
	for i, movie := range movies {
		isLast := i == len(movies)-1
		prefix := "\u251c"
		if isLast {
			prefix = "\u2570"
		}
		
		fmt.Printf("%s\u2500\u2500 %s (%d)\n", prefix, movie.Title, movie.Year)
		
		// Track watch status for warning
		if movie.Watched {
			watchedCount++
		}
		
		indent := "\u2502   "
		if isLast {
			indent = "    "
		}
		
		if len(movie.TagNames) > 0 {
			fmt.Printf("%sTags: %s\n", indent, strings.Join(movie.TagNames, ", "))
		}
		if movie.HasFile {
			fmt.Printf("%sFile: %s\n", indent, movie.Path)
		}
		
		dateInfo := fmt.Sprintf("Added: %s", movie.Added.Format("2006-01-02"))
		if !movie.FileImported.IsZero() {
			dateInfo += fmt.Sprintf(" | Imported: %s", movie.FileImported.Format("2006-01-02"))
		}
		fmt.Printf("%s%s\n", indent, dateInfo)
		
		if movie.WatchCount > 0 {
			watchInfo := fmt.Sprintf("Watched %dx", movie.WatchCount)
			if !movie.LastWatched.IsZero() {
				watchInfo += fmt.Sprintf(" (last: %s)", movie.LastWatched.Format("2006-01-02"))
			}
			fmt.Printf("%s%s\n", indent, watchInfo)
		}
		
		if movie.IsRequested {
			requestInfo := fmt.Sprintf("Requested by: %s", movie.RequestedBy)
			if !movie.RequestDate.IsZero() {
				requestInfo += fmt.Sprintf(" on %s", movie.RequestDate.Format("2006-01-02"))
			}
			if movie.RequestStatus != "" {
				requestInfo += fmt.Sprintf(" (Status: %s)", movie.RequestStatus)
			}
			fmt.Printf("%s%s\n", indent, requestInfo)
		}
		
		if i < len(movies)-1 {
			fmt.Printf("\u2502\n")
		}
	}
	fmt.Println()
	
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
	
	if output == nil || len(output) == 0 {
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

// PrintImportableItems displays importable items for user selection
func (o *Operations) PrintImportableItems(items []*radarr.ManualImportOutput) {
	fmt.Printf("\nFound %d importable file", len(items))
	if len(items) != 1 {
		fmt.Printf("s")
	}
	fmt.Println(":")
	fmt.Println()
	
	for i, item := range items {
		isLast := i == len(items)-1
		prefix := "\u251c"
		if isLast {
			prefix = "\u2570"
		}
		
		movieTitle := "Unknown Movie"
		if item.Movie != nil {
			movieTitle = fmt.Sprintf("%s (%d)", item.Movie.Title, item.Movie.Year)
		}
		
		fmt.Printf("%s\u2500\u2500 %s\n", prefix, item.Path)
		
		indent := "\u2502   "
		if isLast {
			indent = "    "
		}
		
		fmt.Printf("%sMovie: %s\n", indent, movieTitle)
		
		if item.Quality != nil && item.Quality.Quality != nil {
			fmt.Printf("%sQuality: %s\n", indent, item.Quality.Quality.Name)
		}
		
		if item.Size > 0 {
			sizeMB := float64(item.Size) / 1024 / 1024
			fmt.Printf("%sSize: %.2f MB\n", indent, sizeMB)
		}
		
		if len(item.Rejections) > 0 {
			fmt.Printf("%sRejections:\n", indent)
			for _, rejection := range item.Rejections {
				fmt.Printf("%s  - %s\n", indent, rejection.Reason)
			}
		}
		
		if i < len(items)-1 {
			fmt.Printf("\u2502\n")
		}
	}
	fmt.Println()
}