package radarr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

// Client wraps the starr Radarr client with additional functionality
type Client struct {
	api      RadarrAPI
	logger   zerolog.Logger
	
	// Cache for frequently accessed data
	tagCache      []*starr.Tag
	tagCacheMutex sync.RWMutex
	tagCacheTime  time.Time
	cacheTTL      time.Duration
}

// NewClient creates a new Radarr client
func NewClient(url, apiKey string, logger zerolog.Logger) (*Client, error) {
	config := starr.New(apiKey, url, 30*time.Second)
	radarrClient := radarr.New(config)

	// Test the connection
	if err := radarrClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to Radarr: %w", err)
	}

	return &Client{
		api:      radarrClient,
		logger:   logger,
		cacheTTL: 5 * time.Minute,
	}, nil
}

// NewClientWithAPI creates a new client with a custom API implementation (for testing)
func NewClientWithAPI(api RadarrAPI, logger zerolog.Logger) *Client {
	return &Client{
		api:      api,
		logger:   logger,
		cacheTTL: 5 * time.Minute,
	}
}

// GetAllMovies retrieves all movies from Radarr
func (c *Client) GetAllMovies(ctx context.Context) ([]*radarr.Movie, error) {
	movies, err := c.api.GetMovieContext(ctx, &radarr.GetMovie{})
	if err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}

	c.logger.Debug().Msgf("Retrieved %d movies from Radarr", len(movies))
	return movies, nil
}

// GetTags retrieves all tags from Radarr with caching
func (c *Client) GetTags(ctx context.Context) ([]*starr.Tag, error) {
	// Check cache first
	c.tagCacheMutex.RLock()
	if c.tagCache != nil && time.Since(c.tagCacheTime) < c.cacheTTL {
		tags := c.tagCache
		c.tagCacheMutex.RUnlock()
		c.logger.Debug().Msgf("Retrieved %d tags from cache", len(tags))
		return tags, nil
	}
	c.tagCacheMutex.RUnlock()

	// Fetch from API
	tags, err := c.api.GetTagsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	// Update cache
	c.tagCacheMutex.Lock()
	c.tagCache = tags
	c.tagCacheTime = time.Now()
	c.tagCacheMutex.Unlock()

	c.logger.Debug().Msgf("Retrieved %d tags from Radarr", len(tags))
	return tags, nil
}

// DeleteMovie deletes a movie from Radarr
func (c *Client) DeleteMovie(ctx context.Context, movieID int64, deleteFiles bool) error {
	err := c.api.DeleteMovieContext(ctx, movieID, deleteFiles, false)
	if err != nil {
		return fmt.Errorf("failed to delete movie ID %d: %w", movieID, err)
	}

	c.logger.Info().Int64("movie_id", movieID).Bool("delete_files", deleteFiles).
		Msg("Successfully deleted movie")
	return nil
}

// DeleteMovieFiles deletes movie files by their IDs (without deleting the movie entry)
func (c *Client) DeleteMovieFiles(ctx context.Context, movieFileIDs ...int64) error {
	err := c.api.DeleteMovieFilesContext(ctx, movieFileIDs...)
	if err != nil {
		return fmt.Errorf("failed to delete movie files %v: %w", movieFileIDs, err)
	}

	c.logger.Info().Interface("movie_file_ids", movieFileIDs).
		Msg("Successfully deleted movie files")
	return nil
}

// SendCommand sends a command to Radarr
func (c *Client) SendCommand(ctx context.Context, cmd *radarr.CommandRequest) (*radarr.CommandResponse, error) {
	response, err := c.api.SendCommandContext(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to send command %s: %w", cmd.Name, err)
	}

	c.logger.Info().Str("command", cmd.Name).Interface("movie_ids", cmd.MovieIDs).
		Msg("Successfully sent command")
	return response, nil
}

// GetTagByName finds a tag by its label
func (c *Client) GetTagByName(ctx context.Context, tagName string) (*starr.Tag, error) {
	tags, err := c.GetTags(ctx)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if tag.Label == tagName {
			return tag, nil
		}
	}

	return nil, fmt.Errorf("tag not found: %s", tagName)
}

// GetManualImportItems scans a folder for importable movie files
// Note: The starr library's ManualImport method returns a single output, but the actual
// Radarr API returns an array. We need to work around this limitation.
func (c *Client) GetManualImportItems(ctx context.Context, params *radarr.ManualImportParams) ([]*radarr.ManualImportOutput, error) {
	// Unfortunately, the starr library doesn't expose the array response properly
	// We'll use the single item response for now and note this as a limitation
	output, err := c.api.ManualImportContext(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get manual import items: %w", err)
	}

	// Return as array for consistency
	if output != nil {
		c.logger.Debug().Msg("Found 1 item for manual import")
		return []*radarr.ManualImportOutput{output}, nil
	}

	return nil, nil
}

// ProcessManualImport processes manual import of movie files
func (c *Client) ProcessManualImport(ctx context.Context, items []*radarr.ManualImportInput) error {
	// Process each item using the reprocess method
	for _, item := range items {
		err := c.api.ManualImportReprocessContext(ctx, item)
		if err != nil {
			return fmt.Errorf("failed to import file %s: %w", item.Path, err)
		}
		c.logger.Info().Str("path", item.Path).Int64("movie_id", item.MovieID).
			Msg("Successfully imported movie file")
	}

	return nil
}

// MovieInfo contains relevant movie information for filtering and display
type MovieInfo struct {
	ID           int64
	Title        string
	Year         int
	TMDBID       int64
	IMDBID       string
	Path         string
	Tags         []int
	TagNames     []string
	Added        time.Time
	MovieFile    *radarr.MovieFile
	HasFile      bool
	FileImported time.Time
	// Watch status fields (aggregate across all users)
	Watched       bool
	WatchCount    int
	LastWatched   time.Time
	WatchProgress float64
	// Per-user watch data
	UserWatchData map[string]*UserWatchInfo
	// Rating data
	Ratings    map[string]float64 // Map of source -> rating value (e.g. "imdb" -> 7.5)
	Popularity float64
	// Request data from Overseerr
	RequestedBy      string    // Username of who requested the movie
	RequestedByEmail string    // Email of requester
	RequestDate      time.Time // When the movie was requested
	RequestStatus    string    // Status from Overseerr (PENDING, APPROVED, AVAILABLE)
	ApprovedBy       string    // Who approved the request
	IsAutoRequest    bool      // Whether it was an automatic request
	IsRequested      bool      // Whether movie was requested via Overseerr
	// Hardlink data
	HardlinkCount uint32 // Number of hardlinks for the movie file
	IsHardlinked  bool   // Whether file has multiple hardlinks (count > 1)
	// qBittorrent data
	QBittorrentHash string // Hash of matching torrent in qBittorrent
	IsSeeding       bool   // Whether the movie is currently seeding
}

// UserWatchInfo contains watch information for a specific user
type UserWatchInfo struct {
	Username    string
	Watched     bool
	WatchCount  int
	LastWatched time.Time
	MaxProgress float64
}

// GetMovieInfo converts a Radarr movie to our MovieInfo struct
func (c *Client) GetMovieInfo(movie *radarr.Movie, tags []*starr.Tag) MovieInfo {
	info := MovieInfo{
		ID:            movie.ID,
		Title:         movie.Title,
		Year:          movie.Year,
		TMDBID:        movie.TmdbID,
		IMDBID:        movie.ImdbID,
		Path:          movie.Path,
		Tags:          movie.Tags,
		TagNames:      make([]string, 0),
		Added:         movie.Added,
		HasFile:       movie.HasFile,
		UserWatchData: make(map[string]*UserWatchInfo),
		Ratings:       make(map[string]float64),
		Popularity:    movie.Popularity,
	}

	// Map tag IDs to names
	tagMap := make(map[int]string)
	for _, tag := range tags {
		tagMap[tag.ID] = tag.Label
	}

	for _, tagID := range movie.Tags {
		if tagName, ok := tagMap[tagID]; ok {
			info.TagNames = append(info.TagNames, tagName)
		}
	}

	// Get file information
	if movie.MovieFile != nil {
		info.MovieFile = movie.MovieFile
		if !movie.MovieFile.DateAdded.IsZero() {
			info.FileImported = movie.MovieFile.DateAdded
		}
	}

	// Extract ratings
	if movie.Ratings != nil {
		for source, rating := range movie.Ratings {
			if rating.Value > 0 {
				info.Ratings[source] = rating.Value
			}
		}
	}

	return info
}

// GetCustomFormats retrieves all custom formats from Radarr
func (c *Client) GetCustomFormats(ctx context.Context) ([]*radarr.CustomFormatOutput, error) {
	formats, err := c.api.GetCustomFormatsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom formats: %w", err)
	}

	c.logger.Debug().Msgf("Retrieved %d custom formats from Radarr", len(formats))
	return formats, nil
}

// GetMovieFile retrieves detailed movie file information including custom formats
func (c *Client) GetMovieFile(ctx context.Context, fileID int64) (*radarr.MovieFile, error) {
	// Get the movie file details
	movieFile, err := c.api.GetMovieFileByIDContext(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie file ID %d: %w", fileID, err)
	}

	return movieFile, nil
}

// UpdateMovie updates a movie in Radarr (for monitoring status, tags, etc)
func (c *Client) UpdateMovie(ctx context.Context, movie *radarr.Movie) (*radarr.Movie, error) {
	updatedMovie, err := c.api.UpdateMovieContext(ctx, movie.ID, movie, false)
	if err != nil {
		return nil, fmt.Errorf("failed to update movie ID %d: %w", movie.ID, err)
	}

	c.logger.Info().Int64("movie_id", movie.ID).Str("title", movie.Title).
		Bool("monitored", movie.Monitored).
		Msg("Successfully updated movie")
	return updatedMovie, nil
}

// GetMovieByID retrieves a single movie by its ID
func (c *Client) GetMovieByID(ctx context.Context, movieID int64) (*radarr.Movie, error) {
	movie, err := c.api.GetMovieByIDContext(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie ID %d: %w", movieID, err)
	}
	return movie, nil
}
