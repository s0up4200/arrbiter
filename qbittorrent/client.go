package qbittorrent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/autobrr/go-qbittorrent"
	"github.com/rs/zerolog"
)

// Client provides an interface to interact with qBittorrent's API.
// It wraps the underlying qBittorrent client and provides higher-level operations.
type Client struct {
	client *qbittorrent.Client
	logger zerolog.Logger
}

// compile-time check that Client implements expected behavior
var _ interface {
	GetAllTorrents(context.Context) ([]*TorrentInfo, error)
	GetTorrentByPath(context.Context, string) (*TorrentInfo, error)
} = (*Client)(nil)

// NewClient creates a new qBittorrent client with the provided credentials.
// It validates the connection by attempting to log in.
func NewClient(url, username, password string, logger zerolog.Logger) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("qBittorrent URL cannot be empty")
	}
	if username == "" {
		return nil, fmt.Errorf("qBittorrent username cannot be empty")
	}

	// Create client with credentials
	client := qbittorrent.NewClient(qbittorrent.Config{
		Host:     url,
		Username: username,
		Password: password,
	})

	// Test connection by logging in
	if err := client.Login(); err != nil {
		return nil, fmt.Errorf("failed to connect to qBittorrent at %s: %w", url, err)
	}

	logger.Debug().
		Str("url", url).
		Str("username", username).
		Msg("successfully connected to qBittorrent")

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// GetAllTorrents retrieves all torrents from qBittorrent.
// It converts the raw torrent data into TorrentInfo structs.
func (c *Client) GetAllTorrents(ctx context.Context) ([]*TorrentInfo, error) {
	// Check context before making API call
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Get all torrents
	torrents, err := c.client.GetTorrents(qbittorrent.TorrentFilterOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}

	c.logger.Debug().
		Int("count", len(torrents)).
		Msg("retrieved torrents from qBittorrent")

	// Pre-allocate slice for better performance
	results := make([]*TorrentInfo, 0, len(torrents))

	for _, t := range torrents {
		info := convertTorrentInfo(t)
		results = append(results, info)
	}

	return results, nil
}

// GetTorrentByHash retrieves a single torrent by its hash.
func (c *Client) GetTorrentByHash(ctx context.Context, hash string) (*TorrentInfo, error) {
	if hash == "" {
		return nil, fmt.Errorf("torrent hash cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	torrents, err := c.client.GetTorrents(qbittorrent.TorrentFilterOptions{
		Hashes: []string{hash},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent %s: %w", hash, err)
	}

	if len(torrents) == 0 {
		return nil, nil
	}

	return convertTorrentInfo(torrents[0]), nil
}

// convertTorrentInfo converts a qBittorrent torrent to our TorrentInfo model
func convertTorrentInfo(t qbittorrent.Torrent) *TorrentInfo {
	info := &TorrentInfo{
		Hash:           t.Hash,
		Name:           t.Name,
		SavePath:       t.SavePath,
		ContentPath:    t.ContentPath,
		State:          TorrentState(t.State),
		Size:           t.Size,
		Progress:       t.Progress,
		DownloadedSize: t.Downloaded,
		UploadedSize:   t.Uploaded,
		Ratio:          t.Ratio,
		AddedOn:        time.Unix(t.AddedOn, 0),
		CompletionOn:   time.Unix(t.CompletionOn, 0),
		Category:       t.Category,
		Tags:           parseTags(t.Tags),
	}

	// Set seeding status based on state
	info.IsSeeding = info.IsActivelySeeding()

	return info
}

// parseTags splits comma-separated tags and removes empty strings
func parseTags(tags string) []string {
	if tags == "" {
		return nil
	}

	parts := strings.Split(tags, ",")
	result := make([]string, 0, len(parts))

	for _, tag := range parts {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			result = append(result, tag)
		}
	}

	return result
}

// GetTorrentByPath finds a torrent that contains the specified file path.
// It returns nil if no matching torrent is found.
func (c *Client) GetTorrentByPath(ctx context.Context, filePath string) (*TorrentInfo, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	torrents, err := c.GetAllTorrents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}

	// Normalize the search path
	searchPath := filepath.Clean(filePath)

	for _, torrent := range torrents {
		// Check context in loop
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during search: %w", err)
		}

		// Try to match torrent by path
		if matched, err := c.matchTorrentByPath(ctx, torrent, searchPath); err != nil {
			c.logger.Warn().
				Err(err).
				Str("hash", torrent.Hash).
				Str("torrent", torrent.Name).
				Msg("failed to check torrent files")
			continue
		} else if matched {
			return torrent, nil
		}
	}

	return nil, nil
}

// matchTorrentByPath checks if a torrent contains the specified file path
func (c *Client) matchTorrentByPath(ctx context.Context, torrent *TorrentInfo, searchPath string) (bool, error) {
	// Check if the file is within this torrent's path
	torrentPath := filepath.Clean(torrent.GetFullPath())

	// Check exact match first
	if torrentPath == searchPath {
		c.logger.Debug().
			Str("torrent", torrent.Name).
			Str("path", searchPath).
			Msg("found exact match for file in torrent")
		return true, nil
	}

	// Check if file is within torrent directory
	if strings.HasPrefix(searchPath, torrentPath+string(filepath.Separator)) {
		c.logger.Debug().
			Str("torrent", torrent.Name).
			Str("path", searchPath).
			Msg("found file within torrent directory")
		return true, nil
	}

	// For multi-file torrents, check individual files
	files, err := c.GetTorrentFiles(ctx, torrent.Hash)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		fullFilePath := filepath.Join(torrent.SavePath, file)
		if filepath.Clean(fullFilePath) == searchPath {
			c.logger.Debug().
				Str("torrent", torrent.Name).
				Str("file", file).
				Msg("found file in multi-file torrent")
			torrent.Files = append(torrent.Files, file)
			return true, nil
		}
	}

	return false, nil
}

// GetTorrentFiles gets the list of files in a torrent.
// It returns the relative file paths within the torrent.
func (c *Client) GetTorrentFiles(ctx context.Context, hash string) ([]string, error) {
	if hash == "" {
		return nil, fmt.Errorf("torrent hash cannot be empty")
	}

	// Check context before making API call
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	files, err := c.client.GetFilesInformation(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get files for torrent %s: %w", hash, err)
	}

	if files == nil {
		return nil, nil
	}

	// Pre-allocate slice
	filePaths := make([]string, 0, len(*files))
	for _, f := range *files {
		if f.Name != "" {
			filePaths = append(filePaths, f.Name)
		}
	}

	return filePaths, nil
}

// IsTorrentSeeding checks if a specific torrent is seeding.
// It returns false if the torrent is not found.
func (c *Client) IsTorrentSeeding(ctx context.Context, hash string) (bool, error) {
	if hash == "" {
		return false, fmt.Errorf("torrent hash cannot be empty")
	}

	// Check context before making API call
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("context cancelled: %w", err)
	}

	// Get the specific torrent info
	torrents, err := c.client.GetTorrents(qbittorrent.TorrentFilterOptions{
		Hashes: []string{hash},
	})
	if err != nil {
		return false, fmt.Errorf("failed to get torrent %s: %w", hash, err)
	}

	if len(torrents) == 0 {
		return false, nil
	}

	torrent := torrents[0]
	state := TorrentState(torrent.State)
	return state.IsSeeding(), nil
}
