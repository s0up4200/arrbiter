package overseerr

import "time"

// RequestStatus represents the status of a media request
type RequestStatus int

const (
	RequestStatusPending RequestStatus = iota + 1
	RequestStatusApproved
	RequestStatusDeclined
	RequestStatusProcessing
	RequestStatusPartiallyAvailable
	RequestStatusAvailable
	RequestStatusFailed
)

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTv    MediaType = "tv"
)

// User represents an Overseerr user
type User struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username,omitempty"`
	PlexUsername string `json:"plexUsername,omitempty"`
	DisplayName string `json:"displayName"`
	Avatar      string `json:"avatar,omitempty"`
}

// Media represents media information in Overseerr
type Media struct {
	ID              int       `json:"id"`
	TmdbID          int       `json:"tmdbId"`
	TvdbID          int       `json:"tvdbId,omitempty"`
	ImdbID          string    `json:"imdbId,omitempty"`
	Status          int       `json:"status"`
	Status4k        int       `json:"status4k"`
	MediaType       MediaType `json:"mediaType"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	LastSeasonChange time.Time `json:"lastSeasonChange,omitempty"`
	MediaAddedAt    time.Time `json:"mediaAddedAt,omitempty"`
	ServiceID       *int      `json:"serviceId,omitempty"`
	ServiceID4k     *int      `json:"serviceId4k,omitempty"`
	ExternalServiceID *int    `json:"externalServiceId,omitempty"`
	ExternalServiceID4k *int `json:"externalServiceId4k,omitempty"`
	ExternalServiceSlug string `json:"externalServiceSlug,omitempty"`
	ExternalServiceSlug4k string `json:"externalServiceSlug4k,omitempty"`
	RatingKey       string    `json:"ratingKey,omitempty"`
	RatingKey4k     string    `json:"ratingKey4k,omitempty"`
}

// MediaRequest represents a media request in Overseerr
type MediaRequest struct {
	ID          int           `json:"id"`
	Status      RequestStatus `json:"status"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	Type        MediaType     `json:"type"`
	Is4k        bool          `json:"is4k"`
	ServerID    *int          `json:"serverId,omitempty"`
	ProfileID   *int          `json:"profileId,omitempty"`
	RootFolder  *string       `json:"rootFolder,omitempty"`
	LanguageProfileID *int    `json:"languageProfileId,omitempty"`
	Tags        []int         `json:"tags,omitempty"`
	IsAutoRequest bool        `json:"isAutoRequest"`
	RequestedBy User          `json:"requestedBy"`
	ModifiedBy  *User         `json:"modifiedBy,omitempty"`
	Media       Media         `json:"media"`
	SeasonCount int           `json:"seasonCount,omitempty"`
	Seasons     []Season      `json:"seasons,omitempty"`
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

// PageInfo contains pagination information
type PageInfo struct {
	Pages       int `json:"pages"`
	PageSize    int `json:"pageSize"`
	Results     int `json:"results"`
	Page        int `json:"page"`
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