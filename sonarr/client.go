package sonarr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golift.io/starr"
	"golift.io/starr/sonarr"
)

// Client wraps the starr Sonarr client
type Client struct {
	api    *sonarr.Sonarr
	logger zerolog.Logger
}

// NewClient creates a new Sonarr client
func NewClient(url, apiKey string, logger zerolog.Logger) (*Client, error) {
	config := starr.New(apiKey, url, 30*time.Second)
	sonarrClient := sonarr.New(config)

	// Test the connection
	if err := sonarrClient.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to Sonarr: %w", err)
	}

	return &Client{
		api:    sonarrClient,
		logger: logger,
	}, nil
}

// GetAllSeries retrieves all series from Sonarr
func (c *Client) GetAllSeries(ctx context.Context) ([]*sonarr.Series, error) {
	series, err := c.api.GetAllSeriesContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	c.logger.Debug().Msgf("Retrieved %d series from Sonarr", len(series))
	return series, nil
}

// GetTags retrieves all tags from Sonarr
func (c *Client) GetTags(ctx context.Context) ([]*starr.Tag, error) {
	tags, err := c.api.GetTagsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	return tags, nil
}

// DeleteSeries deletes a series and its files from Sonarr
func (c *Client) DeleteSeries(ctx context.Context, seriesID int64) error {
	if err := c.api.DeleteSeriesContext(ctx, int(seriesID), true, false); err != nil {
		return fmt.Errorf("failed to delete series ID %d: %w", seriesID, err)
	}

	c.logger.Info().Int64("series_id", seriesID).Msg("Successfully deleted series")
	return nil
}

// SeriesInfo contains relevant series information for filtering and display
type SeriesInfo struct {
	ID            int64
	Title         string
	Year          int
	TvdbID        int64
	IMDBID        string
	Path          string
	TagNames      []string
	Added         time.Time
	Ended         bool
	Status        string // continuing, ended, upcoming, deleted
	EpisodeCount  int    // Episodes with files on disk
	TotalEpisodes int    // Total episodes known for the series
	SizeOnDisk    int64
	// Watch status fields (aggregate across all users)
	Watched         bool
	WatchedEpisodes int
	WatchCount      int
	LastWatched     time.Time
	WatchProgress   float64 // % of available episodes watched by anyone
	// Per-user watch data
	UserWatchData map[string]*UserWatchInfo
	// Request data from Overseerr
	RequestedBy      string
	RequestedByEmail string
	RequestDate      time.Time
	RequestStatus    string
	ApprovedBy       string
	IsAutoRequest    bool
	IsRequested      bool
}

// UserWatchInfo contains watch information for a specific user
type UserWatchInfo struct {
	Username        string
	Watched         bool // Watched >= min_watch_percent of available episodes
	WatchedEpisodes int
	WatchCount      int
	LastWatched     time.Time
	Progress        float64 // % of available episodes watched (0-100)
}

// GetSeriesInfo converts a Sonarr series to our SeriesInfo struct
func GetSeriesInfo(series *sonarr.Series, tagMap map[int]string) SeriesInfo {
	info := SeriesInfo{
		ID:            series.ID,
		Title:         series.Title,
		Year:          series.Year,
		TvdbID:        series.TvdbID,
		IMDBID:        series.ImdbID,
		Path:          series.Path,
		TagNames:      make([]string, 0),
		Added:         series.Added,
		Ended:         series.Ended,
		Status:        series.Status,
		UserWatchData: make(map[string]*UserWatchInfo),
	}

	for _, tagID := range series.Tags {
		if tagName, ok := tagMap[tagID]; ok {
			info.TagNames = append(info.TagNames, tagName)
		}
	}

	if series.Statistics != nil {
		info.EpisodeCount = series.Statistics.EpisodeFileCount
		info.TotalEpisodes = series.Statistics.TotalEpisodeCount
		info.SizeOnDisk = series.Statistics.SizeOnDisk
	}

	return info
}
