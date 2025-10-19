// Package overseerr provides a client for interacting with the Overseerr API.
//
// Overseerr is a request management and media discovery tool for Plex/Jellyfin/Emby.
// This package implements a clean, idiomatic Go client for fetching movie request data.
//
// # Architecture
//
// The package is organized into several components:
//
//   - Client: The main API client with connection pooling and retry logic
//   - Types: Domain models representing Overseerr entities (requests, media, users)
//   - API: Interface definitions for testability and modularity
//   - Errors: Structured error types for better error handling
//
// # Usage
//
// Create a new client with your Overseerr URL and API key:
//
//	logger := zerolog.New(os.Stdout)
//	client, err := overseerr.NewClient(
//		"https://overseerr.example.com",
//		"your-api-key",
//		logger,
//		overseerr.WithTimeout(30*time.Second),
//		overseerr.WithPageSize(100),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Fetch all movie requests
//	ctx := context.Background()
//	requests, err := client.GetMovieRequests(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Features
//
//   - Context-aware API calls with proper cancellation
//   - Automatic pagination handling
//   - Type-safe request status enums
//   - Structured error types with classification methods
//   - Builder pattern for client configuration
//   - Comprehensive test coverage
//
// # Error Handling
//
// The package defines several error types:
//
//   - ErrInvalidConfig: Invalid client configuration
//   - ErrNoConnection: Connection failure
//   - ErrUnauthorized: Authentication failure
//   - ErrNotFound: Resource not found
//   - APIError: Structured API errors with status codes
//
// API errors include helper methods for classification:
//
//	if apiErr, ok := err.(*overseerr.APIError); ok {
//		if apiErr.IsUnauthorized() {
//			// Handle auth failure
//		}
//	}
package overseerr