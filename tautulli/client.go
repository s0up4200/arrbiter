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

// Client wraps the Tautulli API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewClient creates a new Tautulli client
func NewClient(baseURL, apiKey string, logger zerolog.Logger) (*Client, error) {
	// Ensure base URL ends without slash
	baseURL = strings.TrimRight(baseURL, "/")

	client := &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}

	// Test the connection
	if err := client.TestConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to Tautulli: %w", err)
	}

	return client, nil
}

// TestConnection tests the connection to Tautulli
func (c *Client) TestConnection() error {
	params := url.Values{
		"apikey": {c.apiKey},
		"cmd":    {"get_server_info"},
	}

	requestURL := fmt.Sprintf("%s/api/v2?%s", c.baseURL, params.Encode())
	c.logger.Debug().Str("url", requestURL).Msg("Testing Tautulli connection")

	resp, err := c.httpClient.Get(requestURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Log the response for debugging
	c.logger.Debug().Interface("response", result).Msg("Tautulli response")

	if response, ok := result["response"].(map[string]interface{}); ok {
		if res, ok := response["result"].(string); ok && res == "success" {
			return nil
		}
		// Log what we got instead
		if res, ok := response["result"]; ok {
			c.logger.Error().Interface("result", res).Msg("Unexpected result value")
		}
		if msg, ok := response["message"]; ok {
			return fmt.Errorf("Tautulli error: %v", msg)
		}
	}

	return fmt.Errorf("invalid response from Tautulli: %+v", result)
}

// GetMovieWatchStatus gets the watch status for a movie
func (c *Client) GetMovieWatchStatus(ctx context.Context, imdbID, tmdbID, title string, minWatchPercent float64) (*MovieWatchStatus, error) {
	status := &MovieWatchStatus{
		IMDbID: imdbID,
		TMDbID: tmdbID,
	}

	// Try searching by IMDB ID first
	if imdbID != "" {
		history, err := c.getHistory(ctx, fmt.Sprintf("com.plexapp.agents.imdb://%s", imdbID), "", "movie", "")
		if err == nil && len(history.Response.Data.Data) > 0 {
			c.processHistoryRecords(history.Response.Data.Data, status, minWatchPercent)
			return status, nil
		}
	}

	// Try searching by title
	if title != "" {
		history, err := c.getHistory(ctx, "", title, "movie", "")
		if err == nil && len(history.Response.Data.Data) > 0 {
			// Filter results to match title more precisely
			var matchedRecords []HistoryRecord
			for _, record := range history.Response.Data.Data {
				if strings.EqualFold(record.Title, title) || strings.EqualFold(record.FullTitle, title) {
					matchedRecords = append(matchedRecords, record)
				}
			}
			if len(matchedRecords) > 0 {
				c.processHistoryRecords(matchedRecords, status, minWatchPercent)
				return status, nil
			}
		}
	}

	// No watch history found
	return status, nil
}

// getHistory retrieves history from Tautulli API
func (c *Client) getHistory(ctx context.Context, guid, search, mediaType, user string) (*HistoryResponse, error) {
	params := url.Values{
		"apikey":     {c.apiKey},
		"cmd":        {"get_history"},
		"media_type": {mediaType},
		"length":     {"1000"}, // Get more records
	}

	if guid != "" {
		params.Set("guid", guid)
	}
	if search != "" {
		params.Set("search", search)
	}
	if user != "" {
		params.Set("user", user)
	}

	requestURL := fmt.Sprintf("%s/api/v2?%s", c.baseURL, params.Encode())
	c.logger.Debug().Str("url", requestURL).Msg("Making Tautulli API request")

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var history HistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if history.Response.Result != "success" {
		return nil, fmt.Errorf("API returned error result")
	}

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

// GetMovieWatchStatusWithUsers gets detailed watch status for a movie including per-user data
func (c *Client) GetMovieWatchStatusWithUsers(ctx context.Context, imdbID, tmdbID, title string, minWatchPercent float64) (*MovieWatchStatusWithUsers, error) {
	status := &MovieWatchStatusWithUsers{
		MovieWatchStatus: MovieWatchStatus{
			IMDbID: imdbID,
			TMDbID: tmdbID,
		},
		UserData: make(map[string]*UserWatchData),
	}

	// Get all history for this movie (across all users)
	var allRecords []HistoryRecord

	// Try searching by IMDB ID first
	if imdbID != "" {
		history, err := c.getHistory(ctx, fmt.Sprintf("com.plexapp.agents.imdb://%s", imdbID), "", "movie", "")
		if err == nil && len(history.Response.Data.Data) > 0 {
			allRecords = history.Response.Data.Data
		}
	}

	// If no records found, try searching by title
	if len(allRecords) == 0 && title != "" {
		history, err := c.getHistory(ctx, "", title, "movie", "")
		if err == nil && len(history.Response.Data.Data) > 0 {
			// Filter results to match title more precisely
			for _, record := range history.Response.Data.Data {
				if strings.EqualFold(record.Title, title) || strings.EqualFold(record.FullTitle, title) {
					allRecords = append(allRecords, record)
				}
			}
		}
	}

	// Process records by user
	for _, record := range allRecords {
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
	c.processHistoryRecords(allRecords, &status.MovieWatchStatus, minWatchPercent)

	return status, nil
}

// BatchGetMovieWatchStatus gets watch status for multiple movies efficiently
func (c *Client) BatchGetMovieWatchStatus(ctx context.Context, movies []MovieIdentifier, minWatchPercent float64) (map[string]*MovieWatchStatus, error) {
	results := make(map[string]*MovieWatchStatus)

	// Get all history at once (more efficient than individual queries)
	allHistory, err := c.getHistory(ctx, "", "", "movie", "")
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// Create maps for efficient lookup
	historyByIMDB := make(map[string][]HistoryRecord)
	historyByTitle := make(map[string][]HistoryRecord)

	for _, record := range allHistory.Response.Data.Data {
		if record.IMDbID != "" {
			historyByIMDB[record.IMDbID] = append(historyByIMDB[record.IMDbID], record)
		}
		titleLower := strings.ToLower(record.Title)
		historyByTitle[titleLower] = append(historyByTitle[titleLower], record)
	}

	// Process each movie
	for _, movie := range movies {
		status := &MovieWatchStatus{
			IMDbID: movie.IMDbID,
			TMDbID: strconv.FormatInt(movie.TMDbID, 10),
		}

		// Check by IMDB ID
		if movie.IMDbID != "" {
			if records, ok := historyByIMDB[movie.IMDbID]; ok {
				c.processHistoryRecords(records, status, minWatchPercent)
			}
		}

		// If not found by IMDB, check by title
		if status.WatchCount == 0 && movie.Title != "" {
			titleLower := strings.ToLower(movie.Title)
			if records, ok := historyByTitle[titleLower]; ok {
				c.processHistoryRecords(records, status, minWatchPercent)
			}
		}

		results[movie.IMDbID] = status
	}

	return results, nil
}

// BatchGetMovieWatchStatusWithUsers gets detailed watch status with per-user data for multiple movies
func (c *Client) BatchGetMovieWatchStatusWithUsers(ctx context.Context, movies []MovieIdentifier, minWatchPercent float64) (map[string]*MovieWatchStatusWithUsers, error) {
	results := make(map[string]*MovieWatchStatusWithUsers)

	// Get all history at once (more efficient than individual queries)
	allHistory, err := c.getHistory(ctx, "", "", "movie", "")
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	c.logger.Debug().Int("record_count", len(allHistory.Response.Data.Data)).Msg("Retrieved history records from Tautulli")

	// Create maps for efficient lookup
	historyByIMDB := make(map[string][]HistoryRecord)
	historyByTitle := make(map[string][]HistoryRecord)

	for i, record := range allHistory.Response.Data.Data {
		// Log first few records to see user names
		if i < 5 {
			c.logger.Trace().
				Str("user", record.User).
				Int("user_id", record.UserID).
				Str("title", record.Title).
				Msg("Sample history record")
		}

		if record.IMDbID != "" {
			historyByIMDB[record.IMDbID] = append(historyByIMDB[record.IMDbID], record)
		}
		titleLower := strings.ToLower(record.Title)
		historyByTitle[titleLower] = append(historyByTitle[titleLower], record)
	}

	// Process each movie
	for _, movie := range movies {
		status := &MovieWatchStatusWithUsers{
			MovieWatchStatus: MovieWatchStatus{
				IMDbID: movie.IMDbID,
				TMDbID: strconv.FormatInt(movie.TMDbID, 10),
			},
			UserData: make(map[string]*UserWatchData),
		}

		var relevantRecords []HistoryRecord

		// Check by IMDB ID
		if movie.IMDbID != "" {
			if records, ok := historyByIMDB[movie.IMDbID]; ok {
				relevantRecords = records
			}
		}

		// If not found by IMDB, check by title
		if len(relevantRecords) == 0 && movie.Title != "" {
			titleLower := strings.ToLower(movie.Title)
			if records, ok := historyByTitle[titleLower]; ok {
				relevantRecords = records
			}
		}

		// Process records by user
		for _, record := range relevantRecords {
			username := record.User
			if username == "" {
				continue
			}

			c.logger.Trace().
				Str("movie", movie.Title).
				Str("user", username).
				Int("percent", record.PercentComplete).
				Msg("Processing watch record")

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
		c.processHistoryRecords(relevantRecords, &status.MovieWatchStatus, minWatchPercent)

		results[movie.IMDbID] = status
	}

	return results, nil
}

// MovieIdentifier contains the identifiers for a movie
type MovieIdentifier struct {
	IMDbID string
	TMDbID int64
	Title  string
}
