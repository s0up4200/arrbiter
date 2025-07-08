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
}

// UserWatchInfo contains watch information for a specific user
type UserWatchInfo struct {
	Username      string
	Watched       bool
	WatchCount    int
	LastWatched   time.Time
	MaxProgress   float64
}

// GetMovieInfo converts a Radarr movie to our MovieInfo struct
func (c *Client) GetMovieInfo(movie *radarr.Movie, tags []*starr.Tag) MovieInfo {
	info := MovieInfo{
		ID:       movie.ID,
		Title:    movie.Title,
		Year:     movie.Year,
		TMDBID:   movie.TmdbID,
		IMDBID:   movie.ImdbID,
		Path:     movie.Path,
		Tags:     movie.Tags,
		TagNames:      make([]string, 0),
		Added:         movie.Added,
		HasFile:       movie.HasFile,
		UserWatchData: make(map[string]*UserWatchInfo),
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
		if movie.MovieFile.DateAdded.IsZero() == false {
			info.FileImported = movie.MovieFile.DateAdded
		}
	}
	
	return info
}