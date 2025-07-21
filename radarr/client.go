package radarr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

// Client wraps the starr Radarr client with additional functionality
type Client struct {
	client *radarr.Radarr
	logger zerolog.Logger
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
		client: radarrClient,
		logger: logger,
	}, nil
}

// GetAllMovies retrieves all movies from Radarr
func (c *Client) GetAllMovies(ctx context.Context) ([]*radarr.Movie, error) {
	movies, err := c.client.GetMovieContext(ctx, &radarr.GetMovie{})
	if err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}

	c.logger.Debug().Msgf("Retrieved %d movies from Radarr", len(movies))
	return movies, nil
}

// GetTags retrieves all tags from Radarr
func (c *Client) GetTags(ctx context.Context) ([]*starr.Tag, error) {
	tags, err := c.client.GetTagsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	c.logger.Debug().Msgf("Retrieved %d tags from Radarr", len(tags))
	return tags, nil
}

// DeleteMovie deletes a movie from Radarr
func (c *Client) DeleteMovie(ctx context.Context, movieID int64, deleteFiles bool) error {
	err := c.client.DeleteMovieContext(ctx, movieID, deleteFiles, false)
	if err != nil {
		return fmt.Errorf("failed to delete movie ID %d: %w", movieID, err)
	}

	c.logger.Info().Int64("movie_id", movieID).Bool("delete_files", deleteFiles).
		Msg("Successfully deleted movie")
	return nil
}

// DeleteMovieFiles deletes movie files by their IDs (without deleting the movie entry)
func (c *Client) DeleteMovieFiles(ctx context.Context, movieFileIDs ...int64) error {
	err := c.client.DeleteMovieFilesContext(ctx, movieFileIDs...)
	if err != nil {
		return fmt.Errorf("failed to delete movie files %v: %w", movieFileIDs, err)
	}

	c.logger.Info().Interface("movie_file_ids", movieFileIDs).
		Msg("Successfully deleted movie files")
	return nil
}

// SendCommand sends a command to Radarr
func (c *Client) SendCommand(ctx context.Context, cmd *radarr.CommandRequest) (*radarr.CommandResponse, error) {
	response, err := c.client.SendCommandContext(ctx, cmd)
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
	output, err := c.client.ManualImportContext(ctx, params)
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
		err := c.client.ManualImportReprocessContext(ctx, item)
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
