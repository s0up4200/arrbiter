package filter

import (
	"testing"
	"time"

	"github.com/s0up4200/arrbiter/sonarr"
)

func testSeries() sonarr.SeriesInfo {
	return sonarr.SeriesInfo{
		Title:         "Severance",
		Year:          2022,
		TagNames:      []string{"keep", "01 - Bjorsse"},
		Added:         time.Now().AddDate(0, 0, -90),
		Ended:         false,
		Status:        "continuing",
		EpisodeCount:  18,
		TotalEpisodes: 19,
		UserWatchData: map[string]*sonarr.UserWatchInfo{
			"Johnsa": {
				Username:        "Johnsa",
				Watched:         true,
				WatchedEpisodes: 18,
				WatchCount:      20,
				LastWatched:     time.Now().AddDate(0, 0, -5),
				Progress:        100,
			},
			"ritaaba": {
				Username:        "ritaaba",
				Watched:         false,
				WatchedEpisodes: 2,
				WatchCount:      2,
				LastWatched:     time.Now().AddDate(0, 0, -80),
				Progress:        11.1,
			},
		},
		RequestedBy:   "ritaaba",
		RequestDate:   time.Now().AddDate(0, 0, -100),
		RequestStatus: "AVAILABLE",
		IsRequested:   true,
	}
}

func TestSeriesFilters(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       bool
	}{
		{"watched by user", `watchedBy("Johnsa")`, true},
		{"watched by user case-insensitive", `watchedBy("johnsa")`, true},
		{"not watched by user", `watchedBy("ritaaba")`, false},
		{"watch progress", `watchProgressBy("ritaaba") < 20`, true},
		{"watched episodes", `watchedEpisodesBy("Johnsa") == 18`, true},
		{"last watched stale", `lastWatchedBy("ritaaba") < daysAgo(60)`, true},
		{"last watched recent", `lastWatchedBy("Johnsa") > daysAgo(30)`, true},
		{"never watched user", `lastWatchedBy("nobody").IsZero()`, true},
		{"has tag", `hasTag("keep")`, true},
		{"requested by", `requestedBy("ritaaba")`, true},
		{"requested by case-insensitive", `requestedBy("RITAABA")`, true},
		{"not watched by requester", `notWatchedByRequester()`, true},
		{"watched by requester", `watchedByRequester()`, false},
		{"properties", `Title == "Severance" && EpisodeCount == 18 && !Ended`, true},
		{"abandoned combo", `isRequested() && notWatchedByRequester() && Added < daysAgo(60)`, true},
	}

	series := testSeries()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterFunc, err := ParseAndCreateSeriesFilter(tt.expression)
			if err != nil {
				t.Fatalf("failed to compile %q: %v", tt.expression, err)
			}
			if got := filterFunc(series); got != tt.want {
				t.Errorf("%q = %v, want %v", tt.expression, got, tt.want)
			}
		})
	}
}

func TestSeriesFilterEmptyExpression(t *testing.T) {
	filterFunc, err := ParseAndCreateSeriesFilter("  ")
	if err != nil {
		t.Fatalf("empty expression should compile: %v", err)
	}
	if !filterFunc(testSeries()) {
		t.Error("empty expression should match everything")
	}
}

func TestSeriesFilterInvalidExpression(t *testing.T) {
	for _, expression := range []string{"Title ==", "imdbRating() < 8", "IsHardlinked", "UnknownField == 1"} {
		if _, err := ParseAndCreateSeriesFilter(expression); err == nil {
			t.Errorf("%q must fail to compile", expression)
		}
	}
}
