package tautulli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// defaultTimeout is the default HTTP client timeout.
	defaultTimeout = 30 * time.Second

	// defaultHistoryLimit is the default number of history records to fetch.
	defaultHistoryLimit = 1000

	// imdbGUIDPrefix is the prefix for IMDB GUIDs in Plex.
	imdbGUIDPrefix = "com.plexapp.agents.imdb://"
)

// historyOptions contains parameters for history queries.
type historyOptions struct {
	guid   string
	search string
	user   string
	limit  int
}

// Client provides access to the Tautulli API for retrieving Plex watch history.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewClient creates a new Tautulli client and validates the connection.
// It returns an error if the API key is invalid or the server is unreachable.
func NewClient(baseURL, apiKey string, logger zerolog.Logger) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL cannot be empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("apiKey cannot be empty")
	}

	// Ensure base URL ends without slash
	baseURL = strings.TrimRight(baseURL, "/")

	client := &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
		logger:     logger,
	}

	// Test the connection with a short context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.testConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Tautulli: %w", err)
	}

	return client, nil
}

// testConnection validates the connection to Tautulli by calling get_server_info.
func (c *Client) testConnection(ctx context.Context) error {
	var result struct {
		Response struct {
			Result  string `json:"result"`
			Message string `json:"message"`
		} `json:"response"`
	}

	if err := c.doAPIRequest(ctx, "get_server_info", nil, &result); err != nil {
		return err
	}

	if result.Response.Result != "success" {
		return fmt.Errorf("API returned error: %s", result.Response.Message)
	}

	return nil
}

// doAPIRequest performs a generic API request to Tautulli.
func (c *Client) doAPIRequest(ctx context.Context, cmd string, params url.Values, result interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("apikey", c.apiKey)
	params.Set("cmd", cmd)

	requestURL := fmt.Sprintf("%s/api/v2?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

// GetMovieWatchStatus gets the watch status for a movie.
// Deprecated: Use GetMovieWatchStatusWithUsers for per-user data.
func (c *Client) GetMovieWatchStatus(ctx context.Context, imdbID, tmdbID, title string, minWatchPercent float64) (*MovieWatchStatus, error) {
	status := &MovieWatchStatus{
		IMDbID: imdbID,
		TMDbID: tmdbID,
	}

	records, err := c.findMovieRecords(ctx, imdbID, title)
	if err != nil {
		return status, fmt.Errorf("finding movie records: %w", err)
	}

	c.processHistoryRecords(records, status, minWatchPercent)
	return status, nil
}

// findMovieRecords finds history records for a movie by IMDB ID or title.
func (c *Client) findMovieRecords(ctx context.Context, imdbID, title string) ([]HistoryRecord, error) {
	// Try searching by IMDB ID first
	if imdbID != "" {
		opts := historyOptions{
			guid:  imdbGUIDPrefix + imdbID,
			limit: defaultHistoryLimit,
		}
		history, err := c.getHistory(ctx, opts)
		if err == nil && len(history.Response.Data.Data) > 0 {
			return history.Response.Data.Data, nil
		}
	}

	// Try searching by title
	if title != "" {
		opts := historyOptions{
			search: title,
			limit:  defaultHistoryLimit,
		}
		history, err := c.getHistory(ctx, opts)
		if err != nil {
			return nil, err
		}

		// Filter results to match title more precisely
		var matchedRecords []HistoryRecord
		titleLower := strings.ToLower(title)
		for _, record := range history.Response.Data.Data {
			if strings.ToLower(record.Title) == titleLower || strings.ToLower(record.FullTitle) == titleLower {
				matchedRecords = append(matchedRecords, record)
			}
		}
		return matchedRecords, nil
	}

	return nil, nil
}

// getHistory retrieves history from Tautulli API with the specified filters.
func (c *Client) getHistory(ctx context.Context, opts historyOptions) (*HistoryResponse, error) {
	params := url.Values{
		"media_type": {"movie"},
		"length":     {strconv.Itoa(opts.limit)},
	}

	if opts.guid != "" {
		params.Set("guid", opts.guid)
	}
	if opts.search != "" {
		params.Set("search", opts.search)
	}
	if opts.user != "" {
		params.Set("user", opts.user)
	}

	var history HistoryResponse
	if err := c.doAPIRequest(ctx, "get_history", params, &history); err != nil {
		return nil, fmt.Errorf("get_history API call: %w", err)
	}

	if history.Response.Result != "success" {
		return nil, fmt.Errorf("API returned error: %s", history.Response.Message)
	}

	c.logger.Debug().
		Int("records", len(history.Response.Data.Data)).
		Str("guid", opts.guid).
		Str("search", opts.search).
		Msg("Retrieved history from Tautulli")

	return &history, nil
}

// processHistoryRecords processes history records to determine watch status
func (c *Client) processHistoryRecords(records []HistoryRecord, status *MovieWatchStatus, minWatchPercent float64) {
	for _, record := range records {
		// Update watch count
		status.WatchCount++

		// Check if watched
		if record.IsWatched(minWatchPercent) {
			status.Watched = true
		}

		// Update max progress
		progress := float64(record.PercentComplete)
		if progress > status.MaxProgress {
			status.MaxProgress = progress
		}

		// Update last watched time
		watchTime := record.GetWatchedTime()
		if watchTime.After(status.LastWatched) {
			status.LastWatched = watchTime
		}

		// Store IDs if available
		if status.IMDbID == "" && record.IMDbID != "" {
			status.IMDbID = record.IMDbID
		}
		if status.TMDbID == "" && record.TMDbID != "" {
			status.TMDbID = record.TMDbID
		}
	}
}

// GetMovieWatchStatusWithUsers gets detailed watch status for a movie including per-user data.
func (c *Client) GetMovieWatchStatusWithUsers(ctx context.Context, imdbID, tmdbID, title string, minWatchPercent float64) (*MovieWatchStatusWithUsers, error) {
	status := &MovieWatchStatusWithUsers{
		MovieWatchStatus: MovieWatchStatus{
			IMDbID: imdbID,
			TMDbID: tmdbID,
		},
		UserData: make(map[string]*UserWatchData),
	}

	records, err := c.findMovieRecords(ctx, imdbID, title)
	if err != nil {
		return status, fmt.Errorf("finding movie records: %w", err)
	}

	c.processUserWatchData(records, status, minWatchPercent)
	return status, nil
}

// processUserWatchData processes history records and populates per-user watch data.
func (c *Client) processUserWatchData(records []HistoryRecord, status *MovieWatchStatusWithUsers, minWatchPercent float64) {
	for _, record := range records {
		username := record.User
		if username == "" {
			continue
		}

		// Initialize user data if not exists
		if _, ok := status.UserData[username]; !ok {
			status.UserData[username] = &UserWatchData{
				Username: username,
			}
		}

		userData := status.UserData[username]
		userData.WatchCount++

		// Check if watched
		if record.IsWatched(minWatchPercent) {
			userData.Watched = true
		}

		// Update max progress
		progress := float64(record.PercentComplete)
		if progress > userData.MaxProgress {
			userData.MaxProgress = progress
		}

		// Update last watched time
		watchTime := record.GetWatchedTime()
		if watchTime.After(userData.LastWatched) {
			userData.LastWatched = watchTime
		}
	}

	// Update aggregate status
	c.processHistoryRecords(records, &status.MovieWatchStatus, minWatchPercent)
}

// BatchGetMovieWatchStatus gets watch status for multiple movies efficiently.
// Deprecated: Use BatchGetMovieWatchStatusWithUsers for per-user data.
func (c *Client) BatchGetMovieWatchStatus(ctx context.Context, movies []MovieIdentifier, minWatchPercent float64) (map[string]*MovieWatchStatus, error) {
	allHistory, err := c.getAllHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting all history: %w", err)
	}

	// Build lookup indices
	indices := c.buildHistoryIndices(allHistory.Response.Data.Data)

	results := make(map[string]*MovieWatchStatus, len(movies))
	for _, movie := range movies {
		status := &MovieWatchStatus{
			IMDbID: movie.IMDbID,
			TMDbID: strconv.FormatInt(movie.TMDbID, 10),
		}

		records := c.findRecordsInIndices(movie, indices)
		c.processHistoryRecords(records, status, minWatchPercent)

		results[movie.IMDbID] = status
	}

	return results, nil
}

// BatchGetMovieWatchStatusWithUsers gets detailed watch status with per-user data for multiple movies.
func (c *Client) BatchGetMovieWatchStatusWithUsers(ctx context.Context, movies []MovieIdentifier, minWatchPercent float64) (map[string]*MovieWatchStatusWithUsers, error) {
	allHistory, err := c.getAllHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting all history: %w", err)
	}

	c.logger.Debug().Int("record_count", len(allHistory.Response.Data.Data)).Msg("Retrieved history records from Tautulli")

	// Build lookup indices
	indices := c.buildHistoryIndices(allHistory.Response.Data.Data)

	results := make(map[string]*MovieWatchStatusWithUsers, len(movies))
	for _, movie := range movies {
		status := &MovieWatchStatusWithUsers{
			MovieWatchStatus: MovieWatchStatus{
				IMDbID: movie.IMDbID,
				TMDbID: strconv.FormatInt(movie.TMDbID, 10),
			},
			UserData: make(map[string]*UserWatchData),
		}

		records := c.findRecordsInIndices(movie, indices)
		c.processUserWatchData(records, status, minWatchPercent)

		results[movie.IMDbID] = status
	}

	return results, nil
}

// historyIndices contains pre-built indices for efficient lookups.
type historyIndices struct {
	byIMDB  map[string][]HistoryRecord
	byTitle map[string][]HistoryRecord
}

// getAllHistory fetches all movie history records.
func (c *Client) getAllHistory(ctx context.Context) (*HistoryResponse, error) {
	opts := historyOptions{
		limit: defaultHistoryLimit,
	}
	return c.getHistory(ctx, opts)
}

// buildHistoryIndices creates lookup maps for efficient record finding.
func (c *Client) buildHistoryIndices(records []HistoryRecord) *historyIndices {
	indices := &historyIndices{
		byIMDB:  make(map[string][]HistoryRecord),
		byTitle: make(map[string][]HistoryRecord),
	}

	for _, record := range records {
		if record.IMDbID != "" {
			indices.byIMDB[record.IMDbID] = append(indices.byIMDB[record.IMDbID], record)
		}
		titleLower := strings.ToLower(record.Title)
		indices.byTitle[titleLower] = append(indices.byTitle[titleLower], record)
	}

	return indices
}

// findRecordsInIndices finds records for a movie using pre-built indices.
func (c *Client) findRecordsInIndices(movie MovieIdentifier, indices *historyIndices) []HistoryRecord {
	// Check by IMDB ID first
	if movie.IMDbID != "" {
		if records, ok := indices.byIMDB[movie.IMDbID]; ok {
			return records
		}
	}

	// Check by title
	if movie.Title != "" {
		titleLower := strings.ToLower(movie.Title)
		if records, ok := indices.byTitle[titleLower]; ok {
			return records
		}
	}

	return nil
}
