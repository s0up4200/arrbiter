package tautulli

import (
	"encoding/json"
	"time"
)

// HistoryResponse represents the response from get_history API
type HistoryResponse struct {
	Response Response `json:"response"`
}

// Response contains the actual data and metadata
type Response struct {
	Result string      `json:"result"`
	Data   HistoryData `json:"data"`
}

// HistoryData contains the history records
type HistoryData struct {
	Data []HistoryRecord `json:"data"`
}

// HistoryRecord represents a single history entry
type HistoryRecord struct {
	UserID           int               `json:"user_id"`
	User             string            `json:"user"`
	RatingKey        json.RawMessage   `json:"rating_key"` // Can be string or number
	Title            string            `json:"title"`
	FullTitle        string            `json:"full_title"`
	MediaType        string            `json:"media_type"`
	ThumbURL         string            `json:"thumb"`
	GUID             string            `json:"guid"`
	Date             int64             `json:"date"`
	Started          int64             `json:"started"`
	Stopped          int64             `json:"stopped"`
	Duration         int               `json:"duration"`
	PercentComplete  int               `json:"percent_complete"`
	WatchedStatus    float64           `json:"watched_status"`
	ViewOffset       int               `json:"view_offset"`
	IMDbID           string            `json:"imdb_id"`
	TMDbID           string            `json:"tmdb_id"`
}

// GetWatchedTime returns the time when the item was watched
func (h *HistoryRecord) GetWatchedTime() time.Time {
	if h.Date > 0 {
		return time.Unix(h.Date, 0)
	}
	return time.Time{}
}

// IsWatched checks if the item is considered watched based on percentage
func (h *HistoryRecord) IsWatched(minPercentage float64) bool {
	return float64(h.PercentComplete) >= minPercentage || h.WatchedStatus >= 0.9
}

// MovieWatchStatus contains aggregated watch information for a movie
type MovieWatchStatus struct {
	Watched      bool
	WatchCount   int
	LastWatched  time.Time
	MaxProgress  float64
	IMDbID       string
	TMDbID       string
}

// UserWatchData contains watch information for a specific user
type UserWatchData struct {
	Username     string
	Watched      bool
	WatchCount   int
	LastWatched  time.Time
	MaxProgress  float64
}

// MovieWatchStatusWithUsers contains both aggregate and per-user watch data
type MovieWatchStatusWithUsers struct {
	MovieWatchStatus
	UserData map[string]*UserWatchData
}