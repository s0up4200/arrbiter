package qbittorrent

import "time"

// Option configures a Client.
type Option func(*clientOptions)

// clientOptions holds configuration options for the Client.
type clientOptions struct {
	timeout         time.Duration
	maxRetries      int
	retryDelay      time.Duration
	userAgent       string
	verifyCert      bool
}


// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *clientOptions) {
		o.timeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(retries int) Option {
	return func(o *clientOptions) {
		if retries >= 0 {
			o.maxRetries = retries
		}
	}
}

// WithRetryDelay sets the delay between retry attempts.
func WithRetryDelay(delay time.Duration) Option {
	return func(o *clientOptions) {
		o.retryDelay = delay
	}
}

// WithUserAgent sets a custom user agent string.
func WithUserAgent(userAgent string) Option {
	return func(o *clientOptions) {
		o.userAgent = userAgent
	}
}

// WithInsecureSkipVerify disables certificate verification.
// Use with caution and only for development/testing.
func WithInsecureSkipVerify() Option {
	return func(o *clientOptions) {
		o.verifyCert = false
	}
}