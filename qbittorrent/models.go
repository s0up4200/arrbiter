package qbittorrent

import (
	"path/filepath"
	"time"
)

// TorrentState represents the state of a torrent in qBittorrent.
type TorrentState string

// Torrent states as defined by qBittorrent API.
const (
	StateError              TorrentState = "error"              // Some error occurred
	StateMissingFiles       TorrentState = "missingFiles"       // Torrent data files are missing
	StateUploading          TorrentState = "uploading"          // Torrent is being seeded and data is being transferred
	StatePausedUP           TorrentState = "pausedUP"           // Torrent is paused and has finished downloading
	StateQueuedUP           TorrentState = "queuedUP"           // Queuing is enabled and torrent is queued for upload
	StateStalledUP          TorrentState = "stalledUP"          // Torrent is being seeded, but no connection were made
	StateForcedUP           TorrentState = "forcedUP"           // Torrent is forced to uploading and ignoring queue limit
	StateAllocating         TorrentState = "allocating"         // Torrent is allocating disk space
	StateDownloading        TorrentState = "downloading"        // Torrent is being downloaded and data is being transferred
	StateMetaDL             TorrentState = "metaDL"             // Torrent has just started downloading and is fetching metadata
	StatePausedDL           TorrentState = "pausedDL"           // Torrent is paused and has NOT finished downloading
	StateQueuedDL           TorrentState = "queuedDL"           // Queuing is enabled and torrent is queued for download
	StateStalledDL          TorrentState = "stalledDL"          // Torrent is being downloaded, but no connection were made
	StateForcedDL           TorrentState = "forcedDL"           // Torrent is forced to downloading and ignoring queue limit
	StateCheckingUP         TorrentState = "checkingUP"         // Torrent has finished downloading and is being checked
	StateCheckingDL         TorrentState = "checkingDL"         // Same as checkingUP, but torrent has NOT finished downloading
	StateQueuedForChecking  TorrentState = "queuedForChecking"  // Torrent is queued for checking
	StateCheckingResumeData TorrentState = "checkingResumeData" // Checking resume data on qBt startup
	StateMoving             TorrentState = "moving"             // Torrent is moving to another location
	StateUnknown            TorrentState = "unknown"            // Unknown status
)

// IsSeeding returns true if the torrent state indicates active seeding.
func (s TorrentState) IsSeeding() bool {
	switch s {
	case StateUploading, StateStalledUP, StateQueuedUP, StateForcedUP:
		return true
	default:
		return false
	}
}

// IsDownloading returns true if the torrent state indicates active downloading.
func (s TorrentState) IsDownloading() bool {
	switch s {
	case StateDownloading, StateMetaDL, StateStalledDL, StateForcedDL:
		return true
	default:
		return false
	}
}

// IsActive returns true if the torrent is actively transferring data.
func (s TorrentState) IsActive() bool {
	return s == StateUploading || s == StateDownloading
}

// IsPaused returns true if the torrent is paused.
func (s TorrentState) IsPaused() bool {
	return s == StatePausedUP || s == StatePausedDL
}

// IsError returns true if the torrent has an error.
func (s TorrentState) IsError() bool {
	return s == StateError || s == StateMissingFiles
}

// TorrentInfo contains comprehensive information about a torrent.
type TorrentInfo struct {
	// Identification
	Hash string // Torrent hash (infohash)
	Name string // Torrent name

	// Paths
	SavePath    string   // Directory where torrent contents are saved
	ContentPath string   // Absolute path to torrent contents (file or folder)
	Files       []string // List of files in the torrent (for multi-file torrents)

	// State and progress
	State     TorrentState // Current state of the torrent
	Progress  float64      // Download progress (0.0 to 1.0)
	IsSeeding bool         // Whether the torrent is actively seeding

	// Size and transfer statistics
	Size           int64   // Total size in bytes
	DownloadedSize int64   // Bytes downloaded
	UploadedSize   int64   // Bytes uploaded
	Ratio          float64 // Share ratio

	// Timestamps
	AddedOn      time.Time // When the torrent was added
	CompletionOn time.Time // When the torrent completed downloading

	// Organization
	Category string   // Category name
	Tags     []string // List of tags
}

// IsActivelySeeding checks if the torrent is actively seeding.
func (t *TorrentInfo) IsActivelySeeding() bool {
	return t.State.IsSeeding()
}

// GetFullPath returns the full path to the torrent content.
// It prefers ContentPath if available, otherwise constructs the path
// from SavePath and Name.
func (t *TorrentInfo) GetFullPath() string {
	if t.ContentPath != "" {
		return t.ContentPath
	}
	return filepath.Join(t.SavePath, t.Name)
}

// IsComplete returns true if the torrent has finished downloading.
func (t *TorrentInfo) IsComplete() bool {
	return t.Progress >= 1.0
}

// HasError returns true if the torrent has an error state.
func (t *TorrentInfo) HasError() bool {
	return t.State.IsError()
}

// IsMultiFile returns true if this is a multi-file torrent.
func (t *TorrentInfo) IsMultiFile() bool {
	return len(t.Files) > 1
}

// TorrentMatch represents a candidate torrent match for a given movie.
type TorrentMatch struct {
	Torrent        *TorrentInfo // Reference to the matched torrent
	Score          float64      // Overall match score (0.0 - 1.0)
	TitleMatch     float64      // Title token match ratio (0.0 - 1.0)
	YearMatched    bool         // Whether the torrent name contains the movie year
	SizeDifference int64        // Torrent size minus target size in bytes (if known)
}
