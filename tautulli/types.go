package tautulli

import (
	"encoding/json"
	"strconv"
	"time"
)

// HistoryResponse represents the response from get_history API.
type HistoryResponse struct {
	Response Response `json:"response"`
}

// Response contains the actual data and metadata.
type Response struct {
	Result  string      `json:"result"`
	Message string      `json:"message,omitempty"`
	Data    HistoryData `json:"data"`
}

// HistoryData contains the history records.
type HistoryData struct {
	Data []HistoryRecord `json:"data"`
}

// apiResponse is a generic response structure for API calls.
type apiResponse struct {
	Response struct {
		Result  string `json:"result"`
		Message string `json:"message,omitempty"`
	} `json:"response"`
}

// HistoryRecord represents a single history entry from Tautulli.
type HistoryRecord struct {
	UserID          int             `json:"user_id"`
	User            string          `json:"user"`
	RatingKey       json.RawMessage `json:"rating_key"` // Can be string or number
	Title           string          `json:"title"`
	FullTitle       string          `json:"full_title"`
	MediaType       string          `json:"media_type"`
	ThumbURL        string          `json:"thumb"`
	GUID            string          `json:"guid"`
	Date            int64           `json:"date"`             // Unix timestamp when watched
	Started         int64           `json:"started"`          // Unix timestamp when started
	Stopped         int64           `json:"stopped"`          // Unix timestamp when stopped
	Duration        int             `json:"duration"`         // Duration in seconds
	PercentComplete int             `json:"percent_complete"` // Percentage watched (0-100)
	WatchedStatus   float64         `json:"watched_status"`   // Plex's watched status (0.0-1.0)
	ViewOffset      int             `json:"view_offset"`      // Seconds watched
	IMDbID          string          `json:"imdb_id"`
	TMDbID          string          `json:"tmdb_id"`
}

// GetWatchedTime returns the time when the item was watched.
func (h *HistoryRecord) GetWatchedTime() time.Time {
	if h.Date > 0 {
		return time.Unix(h.Date, 0)
	}
	return time.Time{}
}

// IsWatched checks if the item is considered watched based on percentage.
func (h *HistoryRecord) IsWatched(minPercentage float64) bool {
	return float64(h.PercentComplete) >= minPercentage || h.WatchedStatus >= 0.9
}

// GetRatingKey safely extracts the rating key as a string.
func (h *HistoryRecord) GetRatingKey() string {
	if len(h.RatingKey) == 0 {
		return ""
	}

	// Try to parse as string first
	var str string
	if err := json.Unmarshal(h.RatingKey, &str); err == nil {
		return str
	}

	// Try to parse as number
	var num float64
	if err := json.Unmarshal(h.RatingKey, &num); err == nil {
		return strconv.FormatFloat(num, 'f', -1, 64)
	}

	return ""
}

// MovieWatchStatus contains aggregated watch information for a movie.
type MovieWatchStatus struct {
	Watched     bool      `json:"watched"`
	WatchCount  int       `json:"watch_count"`
	LastWatched time.Time `json:"last_watched"`
	MaxProgress float64   `json:"max_progress"` // Highest percentage watched (0-100)
	IMDbID      string    `json:"imdb_id"`
	TMDbID      string    `json:"tmdb_id"`
}

// UserWatchData contains watch information for a specific user.
type UserWatchData struct {
	Username    string    `json:"username"`
	Watched     bool      `json:"watched"`
	WatchCount  int       `json:"watch_count"`
	LastWatched time.Time `json:"last_watched"`
	MaxProgress float64   `json:"max_progress"` // Highest percentage watched (0-100)
}

// MovieIdentifier contains the identifiers used to look up a movie in Tautulli.
type MovieIdentifier struct {
	IMDbID string // IMDB ID (e.g., "tt1234567")
	TMDbID int64  // The Movie Database ID
	Title  string // Movie title for fallback matching
}

// MovieWatchStatusWithUsers contains both aggregate and per-user watch data.
type MovieWatchStatusWithUsers struct {
	MovieWatchStatus
	UserData map[string]*UserWatchData `json:"user_data"`
}

// IsWatchedByUser checks if a specific user has watched the movie.
func (m *MovieWatchStatusWithUsers) IsWatchedByUser(username string) bool {
	if userData, ok := m.UserData[username]; ok {
		return userData.Watched
	}
	return false
}

// GetUserWatchProgress returns the watch progress for a specific user.
func (m *MovieWatchStatusWithUsers) GetUserWatchProgress(username string) float64 {
	if userData, ok := m.UserData[username]; ok {
		return userData.MaxProgress
	}
	return 0
}

// GetUserWatchCount returns the watch count for a specific user.
func (m *MovieWatchStatusWithUsers) GetUserWatchCount(username string) int {
	if userData, ok := m.UserData[username]; ok {
		return userData.WatchCount
	}
	return 0
}
