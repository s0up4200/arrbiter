package tautulli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
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

	status := processSeriesRecords(records, map[EpisodeKey]bool{{1, 1}: true, {1, 2}: true, {1, 3}: true}, 85)

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

func TestSeriesProgressUsesAvailableEpisodes(t *testing.T) {
	now := time.Now().Unix()
	records := []HistoryRecord{
		episodeRecord("viewer", 1, 1, 100, now),
		episodeRecord("viewer", 2, 1, 95, now-100),
		episodeRecord("viewer", 2, 2, 84, now-200),
	}
	records[2].WatchedStatus = 1
	status := processSeriesRecords(records, map[EpisodeKey]bool{{2, 1}: true, {2, 2}: true}, 85)
	if status.WatchedEpisodes != 1 || status.UserData["viewer"].WatchedEpisodes != 1 {
		t.Fatalf("removed or below-threshold episodes counted as watched: %+v", status)
	}
	if status.LastWatched.Unix() != now || status.WatchCount != 3 {
		t.Fatal("activity on removed episodes must still protect the show")
	}
}

func TestBatchSeriesIdentity(t *testing.T) {
	for _, useIDs := range []bool{true, false} {
		for _, failMetadata := range []bool{false, true} {
			t.Run(strconv.FormatBool(useIDs)+"/metadata_failure="+strconv.FormatBool(failMetadata), func(t *testing.T) {
				first := episodeRecord("viewer", 1, 1, 95, 100)
				first.GrandparentTitle = "Same Show"
				first.GrandparentRatingKey = json.RawMessage(`100`)
				second := episodeRecord("viewer", 1, 1, 20, 200)
				second.GrandparentTitle = "Same Show"
				second.GrandparentRatingKey = json.RawMessage(`"200"`)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var data any
					switch r.URL.Query().Get("cmd") {
					case "get_history":
						data = map[string]any{"data": []HistoryRecord{first, second}}
					case "get_metadata":
						if failMetadata {
							http.Error(w, "metadata unavailable", http.StatusServiceUnavailable)
							return
						}
						key := r.URL.Query().Get("rating_key")
						year, guid := "2020", "tvdb://10"
						if key == "200" {
							year, guid = "2024", "com.plexapp.agents.thetvdb://20?lang=en"
						}
						guids := []string{}
						if useIDs {
							guids = append(guids, guid)
						}
						data = map[string]any{"media_type": "show", "rating_key": key, "title": "Same Show", "year": year, "guids": guids}
					default:
						http.Error(w, "unexpected command", http.StatusBadRequest)
						return
					}
					json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"result": "success", "data": data}})
				}))
				defer server.Close()
				client := &Client{baseURL: server.URL, apiKey: "test-key", httpClient: server.Client(), logger: zerolog.Nop()}
				series := []SeriesIdentifier{
					{ID: 1, TvdbID: 10, Title: "Same Show", Year: 2020, AvailableEpisodes: map[EpisodeKey]bool{{1, 1}: true}},
					{ID: 2, TvdbID: 20, Title: "Same Show", Year: 2024, AvailableEpisodes: map[EpisodeKey]bool{{1, 1}: true}},
					{ID: 3, TvdbID: 30, Title: "Same Show 2", Year: 2020, AvailableEpisodes: map[EpisodeKey]bool{{1, 1}: true}},
				}
				statuses, err := client.BatchGetSeriesWatchStatus(context.Background(), series, 85)
				if failMetadata {
					if err == nil || statuses != nil {
						t.Fatal("metadata failures must not produce empty watch data")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if statuses[1].WatchedEpisodes != 1 || statuses[2].WatchedEpisodes != 0 || statuses[3].WatchCount != 0 {
					t.Fatalf("watch data crossed show identities: %+v", statuses)
				}
			})
		}
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
