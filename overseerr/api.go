package overseerr

import (
	"context"
)

// API defines the interface for Overseerr operations
type API interface {
	// TestConnection verifies the client can connect to Overseerr
	TestConnection(ctx context.Context) error
	
	// GetMovieRequests retrieves all movie requests
	GetMovieRequests(ctx context.Context) ([]MediaRequest, error)
	
	// GetMovieRequestsByTMDBID retrieves movie requests for a specific TMDB ID
	GetMovieRequestsByTMDBID(ctx context.Context, tmdbID int64) ([]MediaRequest, error)
}

// RequestFetcher provides methods for fetching requests with pagination
type RequestFetcher interface {
	// FetchPage fetches a single page of requests
	FetchPage(ctx context.Context, page, pageSize int) (*RequestsResponse, error)
	
	// FetchAll fetches all requests using pagination
	FetchAll(ctx context.Context) ([]MediaRequest, error)
}