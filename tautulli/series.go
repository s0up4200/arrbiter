package tautulli

import (
	"context"
	"fmt"
)

// episodeKey uniquely identifies an episode within a series.
type episodeKey struct {
	season  int
	episode int
}

// BatchGetSeriesWatchStatus gets watch status for multiple series efficiently.
// Results are keyed by the SeriesIdentifier.Title as given.
func (c *Client) BatchGetSeriesWatchStatus(ctx context.Context, series []SeriesIdentifier, minWatchPercent float64) (map[string]*SeriesWatchStatus, error) {
	records, err := c.getAllEpisodeHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting episode history: %w", err)
	}

	// Index records by normalized show title (with and without digits,
	// so "Show (2024)" still matches a Plex title without the year).
	byShow := make(map[string][]HistoryRecord)
	for _, record := range records {
		if record.GrandparentTitle == "" {
			continue
		}
		withDigits, withoutDigits := normalizedVariants(record.GrandparentTitle)
		if withDigits != "" {
			byShow[withDigits] = append(byShow[withDigits], record)
		}
		if withoutDigits != "" && withoutDigits != withDigits {
			byShow[withoutDigits] = append(byShow[withoutDigits], record)
		}
	}

	results := make(map[string]*SeriesWatchStatus, len(series))
	for _, s := range series {
		withDigits, withoutDigits := normalizedVariants(s.Title)
		showRecords, ok := byShow[withDigits]
		if !ok {
			showRecords = byShow[withoutDigits]
		}
		results[s.Title] = processSeriesRecords(showRecords, minWatchPercent)
	}

	return results, nil
}

// getAllEpisodeHistory fetches all episode history records with pagination.
func (c *Client) getAllEpisodeHistory(ctx context.Context) ([]HistoryRecord, error) {
	var (
		allRecords []HistoryRecord
		start      int
	)

	for {
		page, err := c.getHistory(ctx, historyOptions{
			limit:     defaultHistoryLimit,
			start:     start,
			mediaType: "episode",
		})
		if err != nil {
			return nil, err
		}

		records := page.Response.Data.Data
		if len(records) == 0 {
			break
		}

		allRecords = append(allRecords, records...)

		if len(records) < defaultHistoryLimit {
			break
		}
		start += defaultHistoryLimit
	}

	c.logger.Debug().
		Int("records", len(allRecords)).
		Msg("Aggregated Tautulli episode history records")

	return allRecords, nil
}

// processSeriesRecords aggregates episode history records into a SeriesWatchStatus.
func processSeriesRecords(records []HistoryRecord, minWatchPercent float64) *SeriesWatchStatus {
	status := &SeriesWatchStatus{
		UserData: make(map[string]*UserSeriesWatchData),
	}

	watchedByAnyone := make(map[episodeKey]bool)
	watchedByUser := make(map[string]map[episodeKey]bool)

	for _, record := range records {
		season, episode := record.GetSeasonEpisode()
		key := episodeKey{season: season, episode: episode}
		watchTime := record.GetWatchedTime()
		watched := record.IsWatched(minWatchPercent)

		status.WatchCount++
		if watchTime.After(status.LastWatched) {
			status.LastWatched = watchTime
		}
		if watched && !watchedByAnyone[key] {
			watchedByAnyone[key] = true
			status.WatchedEpisodes++
		}

		username := record.User
		if username == "" {
			continue
		}
		userData, ok := status.UserData[username]
		if !ok {
			userData = &UserSeriesWatchData{Username: username}
			status.UserData[username] = userData
			watchedByUser[username] = make(map[episodeKey]bool)
		}
		userData.WatchCount++
		if watchTime.After(userData.LastWatched) {
			userData.LastWatched = watchTime
		}
		if watched && !watchedByUser[username][key] {
			watchedByUser[username][key] = true
			userData.WatchedEpisodes++
		}
	}

	return status
}
