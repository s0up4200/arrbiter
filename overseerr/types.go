package overseerr

import (
	"fmt"
	"time"
)

// RequestStatus represents the status of a media request
type RequestStatus int

const (
	// RequestStatusUnknown represents an unknown request status
	RequestStatusUnknown RequestStatus = iota
	// RequestStatusPending indicates a pending request
	RequestStatusPending
	// RequestStatusApproved indicates an approved request
	RequestStatusApproved
	// RequestStatusDeclined indicates a declined request
	RequestStatusDeclined
	// RequestStatusProcessing indicates a request being processed
	RequestStatusProcessing
	// RequestStatusPartiallyAvailable indicates partial availability
	RequestStatusPartiallyAvailable
	// RequestStatusAvailable indicates full availability
	RequestStatusAvailable
	// RequestStatusFailed indicates a failed request
	RequestStatusFailed
)

// String returns the string representation of a RequestStatus
func (rs RequestStatus) String() string {
	switch rs {
	case RequestStatusPending:
		return "PENDING"
	case RequestStatusApproved:
		return "APPROVED"
	case RequestStatusDeclined:
		return "DECLINED"
	case RequestStatusProcessing:
		return "PROCESSING"
	case RequestStatusPartiallyAvailable:
		return "PARTIALLY_AVAILABLE"
	case RequestStatusAvailable:
		return "AVAILABLE"
	case RequestStatusFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// MediaType represents the type of media
type MediaType string

const (
	// MediaTypeMovie represents a movie
	MediaTypeMovie MediaType = "movie"
	// MediaTypeTV represents a TV show
	MediaTypeTV MediaType = "tv"
)

// IsMovie checks if the media type is a movie
func (mt MediaType) IsMovie() bool {
	return mt == MediaTypeMovie
}

// User represents an Overseerr user
type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username,omitempty"`
	PlexUsername string `json:"plexUsername,omitempty"`
	DisplayName  string `json:"displayName"`
	Avatar       string `json:"avatar,omitempty"`
}

// GetDisplayName returns the best available display name for the user
func (u *User) GetDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Username != "" {
		return u.Username
	}
	if u.PlexUsername != "" {
		return u.PlexUsername
	}
	return u.Email
}

// Media represents media information in Overseerr
type Media struct {
	ID                    int       `json:"id"`
	TmdbID                int       `json:"tmdbId"`
	TvdbID                int       `json:"tvdbId,omitempty"`
	ImdbID                string    `json:"imdbId,omitempty"`
	Status                int       `json:"status"`
	Status4k              int       `json:"status4k"`
	MediaType             MediaType `json:"mediaType"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	LastSeasonChange      time.Time `json:"lastSeasonChange"`
	MediaAddedAt          time.Time `json:"mediaAddedAt"`
	ServiceID             *int      `json:"serviceId,omitempty"`
	ServiceID4k           *int      `json:"serviceId4k,omitempty"`
	ExternalServiceID     *int      `json:"externalServiceId,omitempty"`
	ExternalServiceID4k   *int      `json:"externalServiceId4k,omitempty"`
	ExternalServiceSlug   string    `json:"externalServiceSlug,omitempty"`
	ExternalServiceSlug4k string    `json:"externalServiceSlug4k,omitempty"`
	RatingKey             string    `json:"ratingKey,omitempty"`
	RatingKey4k           string    `json:"ratingKey4k,omitempty"`
}

// GetTMDBID returns the TMDB ID as int64
func (m *Media) GetTMDBID() int64 {
	return int64(m.TmdbID)
}

// MediaRequest represents a media request in Overseerr
type MediaRequest struct {
	ID                int           `json:"id"`
	Status            RequestStatus `json:"status"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
	Type              MediaType     `json:"type"`
	Is4k              bool          `json:"is4k"`
	ServerID          *int          `json:"serverId,omitempty"`
	ProfileID         *int          `json:"profileId,omitempty"`
	RootFolder        *string       `json:"rootFolder,omitempty"`
	LanguageProfileID *int          `json:"languageProfileId,omitempty"`
	Tags              []int         `json:"tags,omitempty"`
	IsAutoRequest     bool          `json:"isAutoRequest"`
	RequestedBy       User          `json:"requestedBy"`
	ModifiedBy        *User         `json:"modifiedBy,omitempty"`
	Media             Media         `json:"media"`
	SeasonCount       int           `json:"seasonCount,omitempty"`
	Seasons           []Season      `json:"seasons,omitempty"`
}

// IsMovieRequest checks if this is a movie request
func (mr *MediaRequest) IsMovieRequest() bool {
	return mr.Type.IsMovie()
}

// GetApprover returns the user who approved the request, if available
func (mr *MediaRequest) GetApprover() *User {
	if mr.ModifiedBy != nil && (mr.Status == RequestStatusApproved || mr.Status == RequestStatusAvailable) {
		return mr.ModifiedBy
	}
	return nil
}

// ToMovieRequest converts a MediaRequest to a simplified MovieRequest
func (mr *MediaRequest) ToMovieRequest() MovieRequest {
	movieReq := MovieRequest{
		RequestedBy:      mr.RequestedBy.GetDisplayName(),
		RequestedByEmail: mr.RequestedBy.Email,
		RequestDate:      mr.CreatedAt,
		RequestStatus:    mr.Status.String(),
		IsAutoRequest:    mr.IsAutoRequest,
	}

	if approver := mr.GetApprover(); approver != nil {
		movieReq.ApprovedBy = approver.GetDisplayName()
	}

	return movieReq
}

// Season represents a TV season request
type Season struct {
	ID           int           `json:"id"`
	SeasonNumber int           `json:"seasonNumber"`
	Status       RequestStatus `json:"status"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// RequestsResponse represents the paginated response from the requests endpoint
type RequestsResponse struct {
	PageInfo PageInfo       `json:"pageInfo"`
	Results  []MediaRequest `json:"results"`
}

// HasMorePages checks if there are more pages to fetch
func (rr *RequestsResponse) HasMorePages() bool {
	return rr.PageInfo.Page < rr.PageInfo.Pages
}

// PageInfo contains pagination information
type PageInfo struct {
	Pages    int `json:"pages"`
	PageSize int `json:"pageSize"`
	Results  int `json:"results"`
	Page     int `json:"page"`
}

// NextPage returns the next page number, or an error if there are no more pages
func (pi *PageInfo) NextPage() (int, error) {
	if pi.Page >= pi.Pages {
		return 0, fmt.Errorf("no more pages available")
	}
	return pi.Page + 1, nil
}

// MovieRequest represents request data specific to our cleanup tool
type MovieRequest struct {
	RequestedBy      string
	RequestedByEmail string
	RequestDate      time.Time
	RequestStatus    string
	ApprovedBy       string
	IsAutoRequest    bool
}