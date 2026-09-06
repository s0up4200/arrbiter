package tautulli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// EpisodeKey identifies an episode within a series.
type EpisodeKey struct {
	Season  int
	Episode int
}

// BatchGetSeriesWatchStatus gets watch status for multiple series efficiently.
// Results use the supplied Sonarr series IDs.
func (c *Client) BatchGetSeriesWatchStatus(ctx context.Context, series []SeriesIdentifier, minWatchPercent float64) (map[int64]*SeriesWatchStatus, error) {
	if len(series) == 0 {
		return map[int64]*SeriesWatchStatus{}, nil
	}
	records, err := c.getAllEpisodeHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting episode history: %w", err)
	}

	byShow := make(map[string][]HistoryRecord)
	for _, record := range records {
		key := parseRatingKey(record.GrandparentRatingKey)
		if key == "" {
			return nil, fmt.Errorf("episode history for %q has no Plex show key", record.GrandparentTitle)
		}
		byShow[key] = append(byShow[key], record)
	}

	bySeries := make(map[int64][]HistoryRecord)
	for key, showRecords := range byShow {
		metadata, err := c.getShowMetadata(ctx, key)
		if err != nil {
			return nil, err
		}
		var matches, fallback []int64
		for _, s := range series {
			if metadata.tvdbID != "" && s.TvdbID != 0 {
				if metadata.tvdbID == strconv.FormatInt(s.TvdbID, 10) {
					matches = append(matches, s.ID)
				}
			} else if metadata.imdbID != "" && s.IMDBID != "" {
				if metadata.imdbID == s.IMDBID {
					matches = append(matches, s.ID)
				}
			} else if s.Year > 0 && s.Year == rawToInt(metadata.Year) &&
				strings.EqualFold(strings.TrimSpace(s.Title), strings.TrimSpace(metadata.Title)) {
				fallback = append(fallback, s.ID)
			}
		}
		if len(matches) == 0 {
			matches = fallback
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("ambiguous Plex show %q (%d)", metadata.Title, rawToInt(metadata.Year))
		}
		for _, id := range matches {
			bySeries[id] = append(bySeries[id], showRecords...)
		}
	}

	results := make(map[int64]*SeriesWatchStatus, len(series))
	for _, s := range series {
		results[s.ID] = processSeriesRecords(bySeries[s.ID], s.AvailableEpisodes, minWatchPercent)
	}

	return results, nil
}

type showMetadata struct {
	MediaType string          `json:"media_type"`
	RatingKey json.RawMessage `json:"rating_key"`
	Title     string          `json:"title"`
	Year      json.RawMessage `json:"year"`
	GUID      string          `json:"guid"`
	GUIDs     []string        `json:"guids"`
	tvdbID    string
	imdbID    string
}

func (c *Client) getShowMetadata(ctx context.Context, key string) (showMetadata, error) {
	var response struct {
		Response struct {
			Result  string       `json:"result"`
			Message string       `json:"message"`
			Data    showMetadata `json:"data"`
		} `json:"response"`
	}
	if err := c.doAPIRequest(ctx, "get_metadata", url.Values{"rating_key": {key}}, &response); err != nil {
		return showMetadata{}, fmt.Errorf("get Plex show %s metadata: %w", key, err)
	}
	metadata := response.Response.Data
	if response.Response.Result != "success" || metadata.MediaType != "show" || parseRatingKey(metadata.RatingKey) != key {
		return showMetadata{}, fmt.Errorf("Plex show %s metadata is unavailable or invalid: %s", key, response.Response.Message)
	}
	for _, guid := range append(metadata.GUIDs, metadata.GUID) {
		parsed, err := url.Parse(guid)
		if err != nil {
			continue
		}
		switch parsed.Scheme {
		case "tvdb", "com.plexapp.agents.thetvdb":
			metadata.tvdbID = parsed.Host
		case "imdb", "com.plexapp.agents.imdb":
			metadata.imdbID = parsed.Host
		}
	}
	if metadata.tvdbID == "" && metadata.imdbID == "" &&
		(strings.TrimSpace(metadata.Title) == "" || rawToInt(metadata.Year) <= 0) {
		return showMetadata{}, fmt.Errorf("Plex show %s has no usable identity", key)
	}
	return metadata, nil
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
func processSeriesRecords(records []HistoryRecord, available map[EpisodeKey]bool, minWatchPercent float64) *SeriesWatchStatus {
	status := &SeriesWatchStatus{
		UserData: make(map[string]*UserSeriesWatchData),
	}

	watchedByAnyone := make(map[EpisodeKey]bool)
	watchedByUser := make(map[string]map[EpisodeKey]bool)

	for _, record := range records {
		season, episode := record.GetSeasonEpisode()
		key := EpisodeKey{Season: season, Episode: episode}
		watchTime := record.GetWatchedTime()
		watched := available[key] && float64(record.PercentComplete) >= minWatchPercent

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
			watchedByUser[username] = make(map[EpisodeKey]bool)
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
