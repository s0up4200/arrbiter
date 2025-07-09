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

// Client wraps the qBittorrent API client
type Client struct {
	client *qbittorrent.Client
	logger zerolog.Logger
}

// NewClient creates a new qBittorrent client
func NewClient(url, username, password string, logger zerolog.Logger) (*Client, error) {
	// Create client with credentials
	client := qbittorrent.NewClient(qbittorrent.Config{
		Host:     url,
		Username: username,
		Password: password,
	})

	// Test connection by logging in
	if err := client.Login(); err != nil {
		return nil, fmt.Errorf("failed to connect to qBittorrent: %w", err)
	}

	//logger.Debug().Msg("Successfully connected to qBittorrent")

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// GetAllTorrents retrieves all torrents from qBittorrent
func (c *Client) GetAllTorrents(ctx context.Context) ([]*TorrentInfo, error) {
	// Get all torrents
	torrents, err := c.client.GetTorrents(qbittorrent.TorrentFilterOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}

	c.logger.Debug().Msgf("Retrieved %d torrents from qBittorrent", len(torrents))

	// Convert to our TorrentInfo struct
	var results []*TorrentInfo
	for _, t := range torrents {
		info := &TorrentInfo{
			Hash:           t.Hash,
			Name:           t.Name,
			SavePath:       t.SavePath,
			ContentPath:    t.ContentPath,
			State:          string(t.State),
			Size:           t.Size,
			Progress:       t.Progress,
			DownloadedSize: t.Downloaded,
			UploadedSize:   t.Uploaded,
			Ratio:          t.Ratio,
			AddedOn:        time.Unix(t.AddedOn, 0),
			CompletionOn:   time.Unix(t.CompletionOn, 0),
			Category:       t.Category,
			Tags:           strings.Split(t.Tags, ","),
			IsSeeding:      false,
		}

		// Check if actively seeding
		info.IsSeeding = info.IsActivelySeeding()

		results = append(results, info)
	}

	return results, nil
}

// GetTorrentByPath finds a torrent that contains the specified file path
func (c *Client) GetTorrentByPath(ctx context.Context, filePath string) (*TorrentInfo, error) {
	torrents, err := c.GetAllTorrents(ctx)
	if err != nil {
		return nil, err
	}

	// Normalize the search path
	searchPath := filepath.Clean(filePath)

	for _, torrent := range torrents {
		// Check if the file is within this torrent's path
		torrentPath := filepath.Clean(torrent.GetFullPath())

		// Check exact match first
		if torrentPath == searchPath {
			c.logger.Debug().
				Str("torrent", torrent.Name).
				Str("path", filePath).
				Msg("Found exact match for file in torrent")
			return torrent, nil
		}

		// Check if file is within torrent directory
		if strings.HasPrefix(searchPath, torrentPath+string(filepath.Separator)) {
			c.logger.Debug().
				Str("torrent", torrent.Name).
				Str("path", filePath).
				Msg("Found file within torrent directory")
			return torrent, nil
		}

		// For multi-file torrents, we need to check individual files
		// This would require getting file information for each torrent
		files, err := c.GetTorrentFiles(ctx, torrent.Hash)
		if err != nil {
			c.logger.Warn().Err(err).Str("hash", torrent.Hash).Msg("Failed to get torrent files")
			continue
		}

		for _, file := range files {
			fullFilePath := filepath.Join(torrent.SavePath, file)
			if filepath.Clean(fullFilePath) == searchPath {
				c.logger.Debug().
					Str("torrent", torrent.Name).
					Str("file", file).
					Msg("Found file in multi-file torrent")
				torrent.Files = append(torrent.Files, file)
				return torrent, nil
			}
		}
	}

	return nil, nil
}

// GetTorrentFiles gets the list of files in a torrent
func (c *Client) GetTorrentFiles(ctx context.Context, hash string) ([]string, error) {
	files, err := c.client.GetFilesInformation(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent files: %w", err)
	}

	var filePaths []string
	if files != nil {
		for _, f := range *files {
			filePaths = append(filePaths, f.Name)
		}
	}

	return filePaths, nil
}

// IsTorrentSeeding checks if a specific torrent is seeding
func (c *Client) IsTorrentSeeding(ctx context.Context, hash string) (bool, error) {
	// Get the specific torrent info
	torrents, err := c.client.GetTorrents(qbittorrent.TorrentFilterOptions{
		Hashes: []string{hash},
	})
	if err != nil {
		return false, fmt.Errorf("failed to get torrent: %w", err)
	}

	if len(torrents) == 0 {
		return false, nil
	}

	torrent := torrents[0]
	// Check various seeding states
	state := string(torrent.State)
	seedingStates := []string{
		"uploading",
		"stalledUP",
		"queuedUP",
		"forcedUP",
	}

	for _, s := range seedingStates {
		if state == s {
			return true, nil
		}
	}

	return false, nil
}
