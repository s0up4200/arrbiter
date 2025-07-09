package qbittorrent

import "time"

// TorrentInfo contains information about a torrent
type TorrentInfo struct {
	Hash            string
	Name            string
	SavePath        string
	ContentPath     string
	State           string
	Size            int64
	Progress        float64
	DownloadedSize  int64
	UploadedSize    int64
	Ratio           float64
	AddedOn         time.Time
	CompletionOn    time.Time
	Category        string
	Tags            []string
	IsSeeding       bool
	Files           []string
}

// IsActivelySeeding checks if the torrent is actively seeding
func (t *TorrentInfo) IsActivelySeeding() bool {
	return t.State == "uploading" || t.State == "stalledUP" || t.State == "queuedUP" || t.State == "forcedUP"
}

// GetFullPath returns the full path to the torrent content
func (t *TorrentInfo) GetFullPath() string {
	if t.ContentPath != "" {
		return t.ContentPath
	}
	return t.SavePath + "/" + t.Name
}