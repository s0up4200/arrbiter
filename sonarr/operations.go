package sonarr

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"

	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/tautulli"
)

// DeleteOptions contains options for deleting series
type DeleteOptions struct {
	DryRun        bool
	ConfirmDelete bool
}

// Operations handles series search and delete operations
type Operations struct {
	client          *Client
	tautulliClient  *tautulli.Client
	overseerrClient *overseerr.Client
	logger          zerolog.Logger
	minWatchPercent float64
}

// NewOperations creates a new Operations instance
func NewOperations(client *Client, logger zerolog.Logger) *Operations {
	return &Operations{
		client:          client,
		logger:          logger,
		minWatchPercent: 85.0,
	}
}

// SetTautulliClient sets the Tautulli client for watch status lookups
func (o *Operations) SetTautulliClient(client *tautulli.Client) {
	o.tautulliClient = client
}

// SetMinWatchPercent sets the minimum watch percentage thresholds
func (o *Operations) SetMinWatchPercent(percent float64) {
	o.minWatchPercent = percent
}

// SetOverseerrClient sets the Overseerr client for request data lookups
func (o *Operations) SetOverseerrClient(client *overseerr.Client) {
	o.overseerrClient = client
}

// GetAllSeries returns all series with enriched data
func (o *Operations) GetAllSeries(ctx context.Context) ([]SeriesInfo, error) {
	series, err := o.client.GetAllSeries(ctx)
	if err != nil {
		return nil, err
	}

	tags, err := o.client.GetTags(ctx)
	if err != nil {
		return nil, err
	}
	tagMap := make(map[int]string, len(tags))
	for _, tag := range tags {
		tagMap[tag.ID] = tag.Label
	}

	results := make([]SeriesInfo, 0, len(series))
	for _, s := range series {
		results = append(results, GetSeriesInfo(s, tagMap))
	}

	if err := o.enrichWatchData(ctx, results); err != nil {
		return nil, fmt.Errorf("enrich series watch data: %w", err)
	}
	if err := o.enrichRequestData(ctx, results); err != nil {
		return nil, fmt.Errorf("enrich series request data: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
	})

	return results, nil
}

// enrichWatchData adds per-user watch status from Tautulli episode history.
func (o *Operations) enrichWatchData(ctx context.Context, series []SeriesInfo) error {
	if o.tautulliClient == nil {
		return nil
	}

	identifiers := make([]tautulli.SeriesIdentifier, 0, len(series))
	for i, s := range series {
		episodes, err := o.client.GetEpisodes(ctx, s.ID)
		if err != nil {
			return err
		}
		available := make(map[tautulli.EpisodeKey]bool)
		for _, episode := range episodes {
			if episode.HasFile && episode.EpisodeFileID > 0 {
				available[tautulli.EpisodeKey{Season: episode.SeasonNumber, Episode: episode.EpisodeNumber}] = true
			}
		}
		series[i].EpisodeCount = len(available)
		identifiers = append(identifiers, tautulli.SeriesIdentifier{
			ID: s.ID, Title: s.Title, Year: s.Year, TvdbID: s.TvdbID, IMDBID: s.IMDBID,
			AvailableEpisodes: available,
		})
	}

	statuses, err := o.tautulliClient.BatchGetSeriesWatchStatus(ctx, identifiers, o.minWatchPercent)
	if err != nil {
		return err
	}

	for i := range series {
		status, ok := statuses[series[i].ID]
		if !ok || status == nil {
			continue
		}

		series[i].WatchedEpisodes = status.WatchedEpisodes
		series[i].WatchCount = status.WatchCount
		series[i].LastWatched = status.LastWatched
		series[i].WatchProgress = episodeProgress(status.WatchedEpisodes, series[i].EpisodeCount)
		series[i].Watched = series[i].EpisodeCount > 0 && series[i].WatchProgress >= o.minWatchPercent

		for username, userData := range status.UserData {
			progress := episodeProgress(userData.WatchedEpisodes, series[i].EpisodeCount)
			series[i].UserWatchData[username] = &UserWatchInfo{
				Username:        userData.Username,
				Watched:         series[i].EpisodeCount > 0 && progress >= o.minWatchPercent,
				WatchedEpisodes: userData.WatchedEpisodes,
				WatchCount:      userData.WatchCount,
				LastWatched:     userData.LastWatched,
				Progress:        progress,
			}
		}
	}

	return nil
}

// episodeProgress returns watched/available as a percentage, 0 when nothing is on disk.
func episodeProgress(watched, available int) float64 {
	if available <= 0 {
		return 0
	}
	progress := float64(watched) / float64(available) * 100
	if progress > 100 {
		progress = 100
	}
	return progress
}

// enrichRequestData adds request information from Overseerr.
func (o *Operations) enrichRequestData(ctx context.Context, series []SeriesInfo) error {
	if o.overseerrClient == nil {
		return nil
	}

	requests, err := o.overseerrClient.GetTVRequests(ctx)
	if err != nil {
		return err
	}

	// Most recent request per TVDB ID
	latestByTvdb := make(map[int64]overseerr.MediaRequest)
	for _, req := range requests {
		tvdbID := int64(req.Media.TvdbID)
		if tvdbID == 0 {
			continue
		}
		if existing, ok := latestByTvdb[tvdbID]; !ok || req.CreatedAt.After(existing.CreatedAt) {
			latestByTvdb[tvdbID] = req
		}
	}

	for i := range series {
		req, ok := latestByTvdb[series[i].TvdbID]
		if !ok {
			continue
		}
		data := req.ToMovieRequest()
		series[i].RequestedBy = data.RequestedBy
		series[i].RequestedByEmail = data.RequestedByEmail
		series[i].RequestDate = data.RequestDate
		series[i].RequestStatus = data.RequestStatus
		series[i].ApprovedBy = data.ApprovedBy
		series[i].IsAutoRequest = data.IsAutoRequest
		series[i].IsRequested = true
	}

	o.logger.Debug().
		Int("total_requests", len(requests)).
		Msg("Enriched series with Overseerr request data")

	return nil
}

// DeleteSeries deletes the given series, honoring dry-run and confirmation settings.
func (o *Operations) DeleteSeries(ctx context.Context, series []SeriesInfo, opts DeleteOptions) error {
	if len(series) == 0 {
		o.logger.Info().Msg("No series to delete")
		return nil
	}

	if opts.DryRun {
		o.logger.Info().Msg("DRY RUN MODE - No series will be deleted")
		fmt.Print(FormatSeriesToDelete(series))
		return nil
	}

	if opts.ConfirmDelete {
		fmt.Print(FormatSeriesToDelete(series))
		if !o.confirmDeletion(len(series)) {
			o.logger.Info().Msg("Deletion cancelled by user")
			return nil
		}
	}

	var failed int
	for _, s := range series {
		if err := o.client.DeleteSeries(ctx, s.ID); err != nil {
			failed++
			o.logger.Error().Err(err).Int64("id", s.ID).Str("title", s.Title).
				Msg("Failed to delete series")
		}
	}

	o.logger.Info().
		Int("deleted", len(series)-failed).
		Int("failed", failed).
		Msg("Series deletion complete")

	if failed > 0 {
		return fmt.Errorf("failed to delete %d series", failed)
	}
	return nil
}

// confirmDeletion prompts the user for confirmation
func (o *Operations) confirmDeletion(count int) bool {
	fmt.Printf("\nAre you sure you want to delete %d series? [y/N]: ", count)

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		response = "n"
	}

	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

// FormatSeriesToDelete renders series in the same tree style as the movie formatter.
func FormatSeriesToDelete(series []SeriesInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nSeries to be deleted (%d):\n\n", len(series))

	for i, s := range series {
		isLast := i == len(series)-1
		prefix, indent := "├", "│   "
		if isLast {
			prefix, indent = "╰", "    "
		}

		fmt.Fprintf(&b, "%s── %s (%d)\n", prefix, s.Title, s.Year)
		fmt.Fprintf(&b, "%sPath: %s\n", indent, s.Path)
		fmt.Fprintf(&b, "%sEpisodes on disk: %d/%d\n", indent, s.EpisodeCount, s.TotalEpisodes)
		if s.WatchCount > 0 {
			fmt.Fprintf(&b, "%sWatched %d episode plays (last: %s)\n", indent, s.WatchCount, s.LastWatched.Format("2006-01-02"))
		}
		if s.IsRequested && s.RequestedBy != "" {
			fmt.Fprintf(&b, "%sRequested by: %s on %s (Status: %s)\n", indent, s.RequestedBy, s.RequestDate.Format("2006-01-02"), s.RequestStatus)
		}
		if !isLast {
			b.WriteString("│\n")
		}
	}

	return b.String()
}
