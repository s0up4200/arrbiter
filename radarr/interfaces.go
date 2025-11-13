package radarr

import (
	"context"

	"golift.io/starr"
	"golift.io/starr/radarr"
)

// RadarrAPI defines the interface for Radarr API operations
type RadarrAPI interface {
	// Movie operations
	GetMovieContext(ctx context.Context, params *radarr.GetMovie) ([]*radarr.Movie, error)
	GetMovieByIDContext(ctx context.Context, movieID int64) (*radarr.Movie, error)
	UpdateMovieContext(ctx context.Context, movieID int64, movie *radarr.Movie, moveFiles bool) (*radarr.Movie, error)
	DeleteMovieContext(ctx context.Context, movieID int64, deleteFiles, addImportExclusion bool) error
	
	// File operations
	GetMovieFileByIDContext(ctx context.Context, fileID int64) (*radarr.MovieFile, error)
	DeleteMovieFilesContext(ctx context.Context, movieFileIDs ...int64) error
	
	// Tag operations
	GetTagsContext(ctx context.Context) ([]*starr.Tag, error)
	
	// Custom format operations
	GetCustomFormatsContext(ctx context.Context) ([]*radarr.CustomFormatOutput, error)
	
	// Command operations
	SendCommandContext(ctx context.Context, cmd *radarr.CommandRequest) (*radarr.CommandResponse, error)
	
	// Import operations
	ManualImportContext(ctx context.Context, params *radarr.ManualImportParams) (*radarr.ManualImportOutput, error)
	ManualImportReprocessContext(ctx context.Context, item *radarr.ManualImportInput) error
	
	// Health check
	Ping() error
}

// MovieEnricher defines the interface for enriching movie data
type MovieEnricher interface {
	EnrichMovies(ctx context.Context, movies []MovieInfo) error
}

// MovieFormatter defines the interface for formatting movie output
type MovieFormatter interface {
	FormatMovieList(movies []MovieInfo, options FormatOptions) string
	FormatMoviesToDelete(movies []MovieInfo) string
	FormatUpgradeCandidates(candidates []UpgradeResult) string
	FormatHardlinkResults(movies []MovieInfo) string
}

// FormatOptions contains options for formatting output
type FormatOptions struct {
	ShowDetails   bool
	ShowWatchInfo bool
	ShowRequests  bool
}