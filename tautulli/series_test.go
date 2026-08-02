package tautulli

import (
	"encoding/json"
	"testing"
	"time"
)

func episodeRecord(user string, season, episode, percent int, date int64) HistoryRecord {
	return HistoryRecord{
		User:             user,
		MediaType:        "episode",
		GrandparentTitle: "Severance",
		ParentMediaIndex: json.RawMessage(jsonInt(season)),
		MediaIndex:       json.RawMessage(jsonInt(episode)),
		PercentComplete:  percent,
		Date:             date,
	}
}

func jsonInt(v int) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestProcessSeriesRecords(t *testing.T) {
	now := time.Now().Unix()
	records := []HistoryRecord{
		// Johnsa watches s1e1 twice (rewatch), s1e2 once
		episodeRecord("Johnsa", 1, 1, 95, now-3000),
		episodeRecord("Johnsa", 1, 1, 100, now-2000),
		episodeRecord("Johnsa", 1, 2, 90, now-1000),
		// ritaaba partially watches s1e1
		episodeRecord("ritaaba", 1, 1, 40, now-500),
		// anonymous record is counted in aggregate only
		episodeRecord("", 1, 3, 95, now-100),
	}

	status := processSeriesRecords(records, 85)

	if status.WatchCount != 5 {
		t.Errorf("WatchCount = %d, want 5", status.WatchCount)
	}
	// s1e1, s1e2, s1e3 watched past threshold by someone
	if status.WatchedEpisodes != 3 {
		t.Errorf("WatchedEpisodes = %d, want 3", status.WatchedEpisodes)
	}

	johnsa := status.UserData["Johnsa"]
	if johnsa == nil {
		t.Fatal("missing Johnsa user data")
	}
	if johnsa.WatchedEpisodes != 2 {
		t.Errorf("Johnsa WatchedEpisodes = %d, want 2 (rewatch must not double-count)", johnsa.WatchedEpisodes)
	}
	if johnsa.WatchCount != 3 {
		t.Errorf("Johnsa WatchCount = %d, want 3", johnsa.WatchCount)
	}

	ritaaba := status.UserData["ritaaba"]
	if ritaaba == nil {
		t.Fatal("missing ritaaba user data")
	}
	if ritaaba.WatchedEpisodes != 0 {
		t.Errorf("ritaaba WatchedEpisodes = %d, want 0 (40%% is below threshold)", ritaaba.WatchedEpisodes)
	}
	if ritaaba.WatchCount != 1 {
		t.Errorf("ritaaba WatchCount = %d, want 1", ritaaba.WatchCount)
	}

	if _, ok := status.UserData[""]; ok {
		t.Error("anonymous user must not get a UserData entry")
	}
}

func TestGetSeasonEpisodeStringNumbers(t *testing.T) {
	record := HistoryRecord{
		ParentMediaIndex: json.RawMessage(`"2"`),
		MediaIndex:       json.RawMessage(`"10"`),
	}
	season, episode := record.GetSeasonEpisode()
	if season != 2 || episode != 10 {
		t.Errorf("GetSeasonEpisode = (%d, %d), want (2, 10)", season, episode)
	}
}
