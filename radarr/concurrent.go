package radarr

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
	"golift.io/starr/radarr"
)

// BatchSize defines the number of items to process concurrently
const (
	DefaultBatchSize = 10
	MaxConcurrency   = 20
)

// ConcurrentProcessor handles concurrent processing of movie operations
type ConcurrentProcessor struct {
	concurrency int
	batchSize   int
}

// NewConcurrentProcessor creates a new concurrent processor with sensible defaults
func NewConcurrentProcessor() *ConcurrentProcessor {
	return &ConcurrentProcessor{
		concurrency: DefaultBatchSize,
		batchSize:   DefaultBatchSize,
	}
}

// ProcessMovieFiles fetches movie file details concurrently
func (c *Client) ProcessMovieFiles(ctx context.Context, movies []*radarr.Movie) error {
	if len(movies) == 0 {
		return nil
	}

	// Create error group with limited concurrency
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(DefaultBatchSize)

	// Use mutex to protect concurrent writes
	var mu sync.Mutex

	for _, movie := range movies {
		// Skip movies without files
		if movie.MovieFile == nil || movie.MovieFile.ID == 0 {
			continue
		}

		currentMovie := movie

		g.Go(func() error {
			// Fetch detailed movie file information
			movieFile, err := c.GetMovieFile(ctx, currentMovie.MovieFile.ID)
			if err != nil {
				c.logger.Warn().
					Err(err).
					Int64("file_id", currentMovie.MovieFile.ID).
					Str("movie", currentMovie.Title).
					Msg("Failed to get movie file details")
				// Continue processing other files
				return nil
			}

			// Update the movie's file information
			mu.Lock()
			currentMovie.MovieFile = movieFile
			mu.Unlock()

			return nil
		})
	}

	return g.Wait()
}

// BatchDeleteMovies deletes movies in batches with proper error aggregation
func (c *Client) BatchDeleteMovies(ctx context.Context, movies []MovieInfo) BatchDeleteResult {
	result := BatchDeleteResult{
		Requested: len(movies),
	}

	if len(movies) == 0 {
		return result
	}

	// Create error group for concurrent deletions
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5) // Lower limit for delete operations

	// Use channels for result collection
	successChan := make(chan int64, len(movies))
	errorChan := make(chan DeleteError, len(movies))

	for _, movie := range movies {
		currentMovie := movie

		g.Go(func() error {
			err := c.DeleteMovie(ctx, currentMovie.ID)
			if err != nil {
				errorChan <- DeleteError{
					MovieID:    currentMovie.ID,
					MovieTitle: currentMovie.Title,
					Err:        err,
				}
			} else {
				successChan <- currentMovie.ID
			}
			return nil // Don't stop on individual errors
		})
	}

	// Wait for all operations to complete
	if err := g.Wait(); err != nil {
		// Even if there's an error, we still want to collect the results
		c.logger.Error().Err(err).Msg("Error during batch delete operations")
	}
	close(successChan)
	close(errorChan)

	// Collect results
	for id := range successChan {
		result.Successful = append(result.Successful, id)
	}
	for err := range errorChan {
		result.Failed = append(result.Failed, err)
	}

	return result
}

// BatchDeleteResult contains the results of a batch delete operation
type BatchDeleteResult struct {
	Requested  int
	Successful []int64
	Failed     []DeleteError
}

// DeleteError contains information about a failed delete operation
type DeleteError struct {
	MovieID    int64
	MovieTitle string
	Err        error // Renamed to avoid conflict with Error() method
}

// Error implements the error interface
func (e DeleteError) Error() string {
	return fmt.Sprintf("failed to delete movie %s (ID: %d): %v", e.MovieTitle, e.MovieID, e.Err)
}

// BatchSearchMovies triggers searches for movies in batches
func (c *Client) BatchSearchMovies(ctx context.Context, movieIDs []int64) error {
	if len(movieIDs) == 0 {
		return nil
	}

	// Process in batches to avoid overwhelming the system
	batchSize := 10
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(3) // Limit concurrent search commands

	for i := 0; i < len(movieIDs); i += batchSize {
		start := i
		end := min(start+batchSize, len(movieIDs))

		batch := movieIDs[start:end]
		batchCopy := batch
		g.Go(func() error {
			searchCommand := &radarr.CommandRequest{
				Name:     "MoviesSearch",
				MovieIDs: batchCopy,
			}

			_, err := c.SendCommand(ctx, searchCommand)
			if err != nil {
				c.logger.Error().
					Err(err).
					Interface("movie_ids", batchCopy).
					Msg("Failed to trigger search for batch")
				// Continue with other batches
				return nil
			}

			c.logger.Info().
				Interface("movie_ids", batchCopy).
				Msg("Successfully triggered search for batch")
			return nil
		})
	}

	return g.Wait()
}

// EnrichMoviesFromMultipleSources enriches movie data from multiple sources concurrently
func (c *Client) EnrichMoviesFromMultipleSources(
	ctx context.Context,
	movies []MovieInfo,
	enrichers ...MovieEnricher,
) error {
	if len(movies) == 0 || len(enrichers) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)

	// Run each enricher concurrently
	for _, enricher := range enrichers {
		enricherCopy := enricher
		g.Go(func() error {
			if err := enricherCopy.EnrichMovies(ctx, movies); err != nil {
				// Log but don't fail the entire operation
				c.logger.Warn().
					Err(err).
					Type("enricher", enricherCopy).
					Msg("Failed to enrich movies")
			}
			return nil
		})
	}

	return g.Wait()
}
