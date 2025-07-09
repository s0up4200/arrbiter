package overseerr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

// Client represents an Overseerr API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewClient creates a new Overseerr client
func NewClient(baseURL, apiKey string, logger zerolog.Logger) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("overseerr URL is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("overseerr API key is required")
	}

	// Ensure baseURL doesn't have trailing slash
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	client := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}

	// Test the connection
	if err := client.TestConnection(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect to Overseerr: %w", err)
	}

	return client, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, endpoint string, params url.Values) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1%s", c.baseURL, endpoint)
	if params != nil && len(params) > 0 {
		url += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	//c.logger.Debug().
	//	Str("method", method).
	//	Str("url", url).
	//	Msg("Making Overseerr API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// TestConnection tests the connection to Overseerr
func (c *Client) TestConnection(ctx context.Context) error {
	// Use the /auth/me endpoint to test connection and API key
	_, err := c.doRequest(ctx, http.MethodGet, "/auth/me", nil)
	if err != nil {
		return err
	}

	//c.logger.Debug().Msg("Successfully connected to Overseerr")
	return nil
}

// GetMovieRequests retrieves all movie requests from Overseerr
func (c *Client) GetMovieRequests(ctx context.Context) ([]MediaRequest, error) {
	var allRequests []MediaRequest
	page := 1
	pageSize := 100

	for {
		params := url.Values{}
		params.Set("take", strconv.Itoa(pageSize))
		params.Set("skip", strconv.Itoa((page-1)*pageSize))
		params.Set("filter", "all") // Get all requests, we'll filter for movies

		body, err := c.doRequest(ctx, http.MethodGet, "/request", params)
		if err != nil {
			return nil, fmt.Errorf("failed to get requests: %w", err)
		}

		var response RequestsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Filter for movie requests only (double-check)
		for _, req := range response.Results {
			if req.Type == MediaTypeMovie {
				allRequests = append(allRequests, req)
			}
		}

		c.logger.Debug().
			Int("page", page).
			Int("count", len(response.Results)).
			Int("total", len(allRequests)).
			Msg("Retrieved movie requests from Overseerr")

		// Check if we've retrieved all pages
		if page >= response.PageInfo.Pages {
			break
		}
		page++
	}

	return allRequests, nil
}

// GetMovieRequestsByTMDBID retrieves movie requests for a specific TMDB ID
func (c *Client) GetMovieRequestsByTMDBID(ctx context.Context, tmdbID int64) ([]MediaRequest, error) {
	// First, we need to find the media ID for this TMDB ID
	// Unfortunately, Overseerr doesn't provide a direct way to filter requests by TMDB ID
	// So we'll get all requests and filter client-side
	allRequests, err := c.GetMovieRequests(ctx)
	if err != nil {
		return nil, err
	}

	var matchingRequests []MediaRequest
	for _, req := range allRequests {
		if int64(req.Media.TmdbID) == tmdbID {
			matchingRequests = append(matchingRequests, req)
		}
	}

	return matchingRequests, nil
}

// ConvertToMovieRequest converts an Overseerr MediaRequest to our simplified MovieRequest
func ConvertToMovieRequest(req MediaRequest) MovieRequest {
	mr := MovieRequest{
		RequestedBy:      req.RequestedBy.DisplayName,
		RequestedByEmail: req.RequestedBy.Email,
		RequestDate:      req.CreatedAt,
		IsAutoRequest:    req.IsAutoRequest,
	}

	// Convert status
	switch req.Status {
	case RequestStatusPending:
		mr.RequestStatus = "PENDING"
	case RequestStatusApproved:
		mr.RequestStatus = "APPROVED"
	case RequestStatusDeclined:
		mr.RequestStatus = "DECLINED"
	case RequestStatusProcessing:
		mr.RequestStatus = "PROCESSING"
	case RequestStatusPartiallyAvailable:
		mr.RequestStatus = "PARTIALLY_AVAILABLE"
	case RequestStatusAvailable:
		mr.RequestStatus = "AVAILABLE"
	case RequestStatusFailed:
		mr.RequestStatus = "FAILED"
	default:
		mr.RequestStatus = "UNKNOWN"
	}

	// Set approved by if available
	if req.ModifiedBy != nil && (req.Status == RequestStatusApproved || req.Status == RequestStatusAvailable) {
		mr.ApprovedBy = req.ModifiedBy.DisplayName
	}

	// Fallback to username if display name is empty
	if mr.RequestedBy == "" && req.RequestedBy.Username != "" {
		mr.RequestedBy = req.RequestedBy.Username
	}
	if mr.RequestedBy == "" && req.RequestedBy.PlexUsername != "" {
		mr.RequestedBy = req.RequestedBy.PlexUsername
	}

	return mr
}
