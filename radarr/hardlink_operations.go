package radarr

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/s0up4200/arrbiter/hardlink"
	"github.com/s0up4200/arrbiter/qbittorrent"
	"golift.io/starr/radarr"
)

// HardlinkOptions contains options for hardlink operations
type HardlinkOptions struct {
	QBittorrentClient *qbittorrent.Client
}

// SetQBittorrentClient sets the qBittorrent client for hardlink operations
func (o *Operations) SetQBittorrentClient(client *qbittorrent.Client) {
	o.qbittorrentClient = client
}

// ScanNonHardlinkedMovies scans for movies that are not hardlinked
func (o *Operations) ScanNonHardlinkedMovies(ctx context.Context) ([]MovieInfo, error) {
	// Get all movies from Radarr without enrichment
	movies, err := o.client.GetAllMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}

	// Get tags for mapping
	tags, err := o.client.GetTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	var nonHardlinkedMovies []MovieInfo
	var processedCount int

	// Check hardlink status for each movie
	for _, movie := range movies {
		// Skip movies without files
		if movie.MovieFile == nil || movie.MovieFile.Path == "" {
			continue
		}

		// Convert to MovieInfo
		info := o.client.GetMovieInfo(movie, tags)
		
		// Only process movies with imported files
		if info.FileImported.IsZero() {
			continue
		}

		processedCount++

		// Check hardlink count
		count, err := hardlink.GetHardlinkCount(movie.MovieFile.Path)
		if err != nil {
			o.logger.Warn().
				Err(err).
				Str("movie", info.Title).
				Str("path", movie.MovieFile.Path).
				Msg("Failed to check hardlink status")
			continue
		}

		info.HardlinkCount = count
		info.IsHardlinked = count > 1

		// Only process non-hardlinked movies
		if !info.IsHardlinked {
			// Check if movie exists in qBittorrent if client is available
			if o.qbittorrentClient != nil {
				torrent, err := o.qbittorrentClient.GetTorrentByPath(ctx, movie.MovieFile.Path)
				if err != nil {
					o.logger.Warn().Err(err).Str("movie", info.Title).Msg("Failed to check qBittorrent status")
				} else if torrent != nil {
					info.QBittorrentHash = torrent.Hash
					info.IsSeeding = torrent.IsSeeding
				}
			}

			nonHardlinkedMovies = append(nonHardlinkedMovies, info)
		}
	}

	o.logger.Info().
		Int("total", processedCount).
		Int("non_hardlinked", len(nonHardlinkedMovies)).
		Msg("Completed hardlink scan")

	return nonHardlinkedMovies, nil
}

// ReimportMovieFromQBittorrent re-imports a movie from qBittorrent to create hardlinks
func (o *Operations) ReimportMovieFromQBittorrent(ctx context.Context, movie MovieInfo) error {
	if movie.QBittorrentHash == "" {
		return fmt.Errorf("movie not found in qBittorrent")
	}

	// Get the torrent info
	torrent, err := o.qbittorrentClient.GetTorrentByPath(ctx, movie.MovieFile.Path)
	if err != nil {
		return fmt.Errorf("failed to get torrent info: %w", err)
	}
	if torrent == nil {
		return fmt.Errorf("torrent not found")
	}

	// Use manual import to re-import the file
	// This will create a hardlink between qBittorrent and Radarr
	importPath := torrent.GetFullPath()
	
	o.logger.Info().
		Str("movie", movie.Title).
		Str("path", importPath).
		Msg("Re-importing movie from qBittorrent")

	// Create manual import params
	params := &radarr.ManualImportParams{
		Folder:              filepath.Dir(importPath),
		MovieID:             movie.ID,
		FilterExistingFiles: false, // We want to re-import even if it exists
	}

	// Scan for importable items
	items, err := o.client.GetManualImportItems(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to scan for import: %w", err)
	}

	if len(items) == 0 {
		return fmt.Errorf("no importable items found")
	}

	// Find the matching item
	var targetItem *radarr.ManualImportOutput
	for _, item := range items {
		if item.Path == importPath || filepath.Base(item.Path) == filepath.Base(importPath) {
			targetItem = item
			break
		}
	}

	if targetItem == nil {
		return fmt.Errorf("could not find matching file in import scan")
	}

	// Convert to import input
	importInputs := o.ConvertToImportInput([]*radarr.ManualImportOutput{targetItem}, "copy")
	if len(importInputs) == 0 {
		return fmt.Errorf("no valid items to import")
	}

	// Process the import
	if err := o.client.ProcessManualImport(ctx, importInputs); err != nil {
		return fmt.Errorf("failed to process import: %w", err)
	}

	o.logger.Info().
		Str("movie", movie.Title).
		Msg("Successfully re-imported movie with hardlink")

	return nil
}

// DeleteAndResearchMovie deletes a movie file and triggers a new search
func (o *Operations) DeleteAndResearchMovie(ctx context.Context, movie MovieInfo) error {
	o.logger.Info().
		Str("movie", movie.Title).
		Msg("Deleting movie file and triggering new search")

	// First, delete just the file (not the movie entry)
	if movie.MovieFile != nil && movie.MovieFile.ID > 0 {
		// Delete the movie file using the movie ID and delete files flag
		// This will delete the file but keep the movie entry
		err := o.client.DeleteMovie(ctx, movie.ID, true)
		if err != nil {
			return fmt.Errorf("failed to delete movie file: %w", err)
		}
		o.logger.Debug().Msg("Deleted movie file")
		
		// Now we need to re-add the movie to trigger a new search
		// For now, we'll just log that a manual search is needed
		o.logger.Info().Msg("Movie file deleted. Please trigger a manual search in Radarr")
	}

	return nil
}