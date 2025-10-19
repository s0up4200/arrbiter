package overseerr

import (
	"errors"
	"fmt"
)

// Common errors
var (
	// ErrInvalidConfig indicates invalid client configuration
	ErrInvalidConfig = errors.New("invalid overseerr configuration")
	// ErrNoConnection indicates connection failure
	ErrNoConnection = errors.New("failed to connect to overseerr")
	// ErrUnauthorized indicates authentication failure
	ErrUnauthorized = errors.New("unauthorized: invalid API key")
	// ErrNotFound indicates resource not found
	ErrNotFound = errors.New("resource not found")
)

// APIError represents an Overseerr API error
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("overseerr API error: status %d: %s", e.StatusCode, e.Message)
}

// IsNotFound checks if the error indicates a not found response
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsUnauthorized checks if the error indicates an authentication failure
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}