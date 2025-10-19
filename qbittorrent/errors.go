package qbittorrent

import "errors"

// Common errors returned by the qBittorrent client.
var (
	// ErrTorrentNotFound is returned when a torrent is not found.
	ErrTorrentNotFound = errors.New("torrent not found")

	// ErrInvalidHash is returned when a torrent hash is invalid.
	ErrInvalidHash = errors.New("invalid torrent hash")

	// ErrConnectionFailed is returned when connection to qBittorrent fails.
	ErrConnectionFailed = errors.New("connection to qBittorrent failed")
)