// Package qbittorrent provides a client for interacting with the qBittorrent Web API.
//
// This package wraps the autobrr/go-qbittorrent library to provide a higher-level
// interface tailored for the arrbiter application's needs, particularly for
// managing hardlinks between qBittorrent and media management applications.
//
// # Features
//
//   - Connection management with authentication
//   - Torrent retrieval and filtering
//   - File path matching for torrent identification
//   - Seeding status detection
//   - Context-aware operations for graceful cancellation
//
// # Usage
//
//	client, err := qbittorrent.NewClient(url, username, password, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get all torrents
//	torrents, err := client.GetAllTorrents(ctx)
//
//	// Find torrent by file path
//	torrent, err := client.GetTorrentByPath(ctx, "/path/to/file")
//	if torrent != nil && torrent.IsActivelySeeding() {
//	    // Handle seeding torrent
//	}
package qbittorrent