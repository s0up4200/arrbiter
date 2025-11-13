package tautulli

import "errors"

// Common errors returned by the Tautulli client.
var (
	// ErrInvalidResponse indicates the API returned an unexpected response format.
	ErrInvalidResponse = errors.New("invalid response from Tautulli API")
	
	// ErrNoHistory indicates no history was found for the given criteria.
	ErrNoHistory = errors.New("no history found")
	
	// ErrAPIFailure indicates the API returned a failure status.
	ErrAPIFailure = errors.New("Tautulli API returned failure status")
)