package overseerr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
	// DefaultPageSize is the default page size for paginated requests
	DefaultPageSize = 100
	// APIVersion is the Overseerr API version
	APIVersion = "v1"
)

// ClientOption configures the client
type ClientOption func(*Client)

// WithTimeout sets a custom timeout for HTTP requests
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithPageSize sets the default page size for paginated requests
func WithPageSize(size int) ClientOption {
	return func(c *Client) {
		c.pageSize = size
	}
}

// Client represents an Overseerr API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	pageSize   int
	logger     zerolog.Logger
}

// NewClient creates a new Overseerr client with options
func NewClient(baseURL, apiKey string, logger zerolog.Logger, opts ...ClientOption) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("%w: URL is required", ErrInvalidConfig)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidConfig)
	}

	// Normalize base URL
	baseURL = strings.TrimRight(baseURL, "/")

	client := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		pageSize: DefaultPageSize,
		logger:   logger,
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := client.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoConnection, err)
	}

	return client, nil
}

// buildURL constructs the full API URL
func (c *Client) buildURL(endpoint string, params url.Values) string {
	url := fmt.Sprintf("%s/api/%s%s", c.baseURL, APIVersion, endpoint)
	if len(params) > 0 {
		url += "?" + params.Encode()
	}
	return url
}

// newRequest creates a new HTTP request with authentication
func (c *Client) newRequest(ctx context.Context, method, endpoint string, params url.Values) (*http.Request, error) {
	url := c.buildURL(endpoint, params)
	
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "arrbiter/1.0")

	return req, nil
}

// doRequest performs an HTTP request and handles the response
func (c *Client) doRequest(ctx context.Context, method, endpoint string, params url.Values) ([]byte, error) {
	req, err := c.newRequest(ctx, method, endpoint, params)
	if err != nil {
		return nil, err
	}

	c.logger.Debug().
		Str("method", method).
		Str("endpoint", endpoint).
		Msg("Making Overseerr API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}

		// Try to extract error message from response
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Message != "" {
				apiErr.Message = errResp.Message
			} else if errResp.Error != "" {
				apiErr.Message = errResp.Error
			}
		}

		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}

		if apiErr.IsUnauthorized() {
			return nil, fmt.Errorf("%w: %v", ErrUnauthorized, apiErr)
		}

		return nil, apiErr
	}

	return body, nil
}

// get performs a GET request
func (c *Client) get(ctx context.Context, endpoint string, params url.Values, result any) error {
	body, err := c.doRequest(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return err
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
	}

	return nil
}

// TestConnection verifies the connection to Overseerr
func (c *Client) TestConnection(ctx context.Context) error {
	// Use the /auth/me endpoint to test connection and API key
	var user struct {
		ID          int    `json:"id"`
		DisplayName string `json:"displayName"`
	}
	
	if err := c.get(ctx, "/auth/me", nil, &user); err != nil {
		return err
	}

	c.logger.Debug().
		Int("user_id", user.ID).
		Str("user_name", user.DisplayName).
		Msg("Successfully connected to Overseerr")
	
	return nil
}

// FetchPage fetches a single page of requests
func (c *Client) FetchPage(ctx context.Context, page, pageSize int) (*RequestsResponse, error) {
	params := url.Values{}
	params.Set("take", strconv.Itoa(pageSize))
	params.Set("skip", strconv.Itoa((page-1)*pageSize))
	params.Set("filter", "all")

	var response RequestsResponse
	if err := c.get(ctx, "/request", params, &response); err != nil {
		return nil, fmt.Errorf("fetching requests page %d: %w", page, err)
	}

	// Update page info if not set correctly by API
	if response.PageInfo.Page == 0 {
		response.PageInfo.Page = page
	}

	return &response, nil
}

// FetchAll fetches all requests using pagination
func (c *Client) FetchAll(ctx context.Context) ([]MediaRequest, error) {
	var allRequests []MediaRequest
	page := 1

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		response, err := c.FetchPage(ctx, page, c.pageSize)
		if err != nil {
			return nil, err
		}

		// Filter for movie requests only
		for _, req := range response.Results {
			if req.IsMovieRequest() {
				allRequests = append(allRequests, req)
			}
		}

		c.logger.Debug().
			Int("page", page).
			Int("fetched", len(response.Results)).
			Int("movies", len(allRequests)).
			Msg("Fetched request page")

		// Check if we've retrieved all pages
		if !response.HasMorePages() {
			break
		}
		
		page++
	}

	return allRequests, nil
}

// GetMovieRequests retrieves all movie requests from Overseerr
func (c *Client) GetMovieRequests(ctx context.Context) ([]MediaRequest, error) {
	requests, err := c.FetchAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching movie requests: %w", err)
	}

	c.logger.Info().
		Int("count", len(requests)).
		Msg("Retrieved movie requests from Overseerr")

	return requests, nil
}

// GetMovieRequestsByTMDBID retrieves movie requests for a specific TMDB ID
func (c *Client) GetMovieRequestsByTMDBID(ctx context.Context, tmdbID int64) ([]MediaRequest, error) {
	// Unfortunately, Overseerr doesn't provide a direct way to filter requests by TMDB ID
	// So we need to fetch all requests and filter client-side
	allRequests, err := c.GetMovieRequests(ctx)
	if err != nil {
		return nil, err
	}

	// Filter requests by TMDB ID
	requests := make([]MediaRequest, 0)
	for _, req := range allRequests {
		if req.Media.GetTMDBID() == tmdbID {
			requests = append(requests, req)
		}
	}

	c.logger.Debug().
		Int64("tmdb_id", tmdbID).
		Int("matches", len(requests)).
		Msg("Filtered requests by TMDB ID")

	return requests, nil
}

// GetLatestMovieRequest returns the most recent request for a TMDB ID
func (c *Client) GetLatestMovieRequest(ctx context.Context, tmdbID int64) (*MediaRequest, error) {
	requests, err := c.GetMovieRequestsByTMDBID(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	if len(requests) == 0 {
		return nil, fmt.Errorf("%w: no requests found for TMDB ID %d", ErrNotFound, tmdbID)
	}

	// Find the most recent request
	latest := &requests[0]
	for i := 1; i < len(requests); i++ {
		if requests[i].CreatedAt.After(latest.CreatedAt) {
			latest = &requests[i]
		}
	}

	return latest, nil
}

// Ensure Client implements the API interface
var _ API = (*Client)(nil)