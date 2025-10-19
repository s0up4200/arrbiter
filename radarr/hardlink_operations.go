package radarr

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/sync/errgroup"
	"golift.io/starr/radarr"

	"github.com/s0up4200/arrbiter/hardlink"
	"github.com/s0up4200/arrbiter/qbittorrent"
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

	// Process movies concurrently for hardlink checking
	results := make(chan MovieInfo, len(movies))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(DefaultBatchSize)

	for _, movie := range movies {
		movie := movie
		// Skip movies without files
		if movie.MovieFile == nil || movie.MovieFile.Path == "" {
			continue
		}

		g.Go(func() error {
			// Convert to MovieInfo
			info := o.client.GetMovieInfo(movie, tags)

			// Only process movies with imported files
			if info.FileImported.IsZero() {
				return nil
			}

			// Check hardlink count
			count, err := hardlink.GetHardlinkCount(movie.MovieFile.Path)
			if err != nil {
				o.logger.Warn().
					Err(err).
					Str("movie", info.Title).
					Str("path", movie.MovieFile.Path).
					Msg("Failed to check hardlink status")
				return nil // Continue processing other movies
			}

			info.HardlinkCount = count
			info.IsHardlinked = count > 1

			// Only process non-hardlinked movies
			if !info.IsHardlinked {
				if o.qbittorrentClient != nil {
					// Check if movie exists in qBittorrent using original path
					torrent, err := o.qbittorrentClient.GetTorrentByPath(ctx, movie.MovieFile.Path)
					if err != nil {
						o.logger.Warn().Err(err).Str("movie", info.Title).Msg("Failed to check qBittorrent status")
					} else if torrent != nil {
						info.QBittorrentHash = torrent.Hash
						info.IsSeeding = torrent.IsSeeding
					} else {
						// Attempt to locate potential alternate torrents for re-import
						targetSize := int64(0)
						if movie.MovieFile != nil {
							targetSize = movie.MovieFile.Size
						}

						matches, err := o.qbittorrentClient.FindAlternateTorrents(ctx, info.Title, info.Year, targetSize)
						if err != nil {
							o.logger.Warn().Err(err).Str("movie", info.Title).Msg("Failed to search alternate torrents")
						} else if len(matches) > 0 {
							info.AlternateTorrents = matches
						}
					}
				}

				results <- info
			}

			return nil
		})
	}

	// Wait for all goroutines to complete
	go func() {
		if err := g.Wait(); err != nil {
			o.logger.Error().Err(err).Msg("Error during hardlink checking")
		}
		close(results)
	}()

	// Collect results
	for info := range results {
		nonHardlinkedMovies = append(nonHardlinkedMovies, info)
		processedCount++
	}

	o.logger.Info().
		Int("total", processedCount).
		Int("non_hardlinked", len(nonHardlinkedMovies)).
		Msg("Completed hardlink scan")

	return nonHardlinkedMovies, nil
}

// ReimportMovieFromQBittorrent re-imports a movie from qBittorrent to create hardlinks
func (o *Operations) ReimportMovieFromQBittorrent(ctx context.Context, movie MovieInfo) error {
	if o.qbittorrentClient == nil {
		return fmt.Errorf("qBittorrent client is not configured")
	}
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

	return o.reimportMovieFromTorrent(ctx, movie, torrent)
}

// ReimportMovieFromTorrentMatch re-imports a movie using the provided torrent match.
func (o *Operations) ReimportMovieFromTorrentMatch(ctx context.Context, movie MovieInfo, match *qbittorrent.TorrentMatch) error {
	if o.qbittorrentClient == nil {
		return fmt.Errorf("qBittorrent client is not configured")
	}
	if match == nil || match.Torrent == nil {
		return fmt.Errorf("invalid torrent match")
	}

	torrent, err := o.qbittorrentClient.GetTorrentByHash(ctx, match.Torrent.Hash)
	if err != nil {
		return fmt.Errorf("failed to get torrent info: %w", err)
	}
	if torrent == nil {
		return fmt.Errorf("torrent not found")
	}

	return o.reimportMovieFromTorrent(ctx, movie, torrent)
}

func (o *Operations) reimportMovieFromTorrent(ctx context.Context, movie MovieInfo, torrent *qbittorrent.TorrentInfo) error {
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
		// Delete only the movie file using its file ID
		// This keeps the movie entry in Radarr but removes the file
		err := o.client.DeleteMovieFiles(ctx, movie.MovieFile.ID)
		if err != nil {
			return fmt.Errorf("failed to delete movie file: %w", err)
		}
		o.logger.Debug().Int64("file_id", movie.MovieFile.ID).Msg("Deleted movie file")

		// Trigger a search for a new version of the movie
		searchCommand := &radarr.CommandRequest{
			Name:     "MoviesSearch",
			MovieIDs: []int64{movie.ID},
		}

		_, err = o.client.SendCommand(ctx, searchCommand)
		if err != nil {
			return fmt.Errorf("failed to trigger movie search: %w", err)
		}

		o.logger.Info().
			Str("movie", movie.Title).
			Int64("movie_id", movie.ID).
			Msg("Successfully deleted movie file and triggered search")
	}

	return nil
}
