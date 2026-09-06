package sonarr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/tautulli"
	"golift.io/starr"
	starrsonarr "golift.io/starr/sonarr"
)

func TestSeriesEnrichmentSafety(t *testing.T) {
	for _, failure := range []string{"", "episodes", "history", "requests"} {
		t.Run("failure="+failure, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected mutation: %s", r.Method)
				}
				switch r.URL.Path {
				case "/api/v3/series":
					fmt.Fprint(w, `[{"id":1,"title":"Example Show","year":2020,"tvdbId":10,"statistics":{"episodeFileCount":1,"totalEpisodeCount":3}}]`)
				case "/api/v3/tag":
					fmt.Fprint(w, `[]`)
				case "/api/v3/episode":
					if failure == "episodes" {
						http.Error(w, "episode lookup failed", http.StatusServiceUnavailable)
						return
					}
					// Two episodes share one file. The old season has no files.
					fmt.Fprint(w, `[{"seasonNumber":1,"episodeNumber":1},{"seasonNumber":2,"episodeNumber":1,"hasFile":true,"episodeFileId":10},{"seasonNumber":2,"episodeNumber":2,"hasFile":true,"episodeFileId":10}]`)
				case "/api/v1/auth/me":
					fmt.Fprint(w, `{"id":1}`)
				case "/api/v1/request":
					if failure == "requests" {
						http.Error(w, "request lookup failed", http.StatusServiceUnavailable)
						return
					}
					fmt.Fprint(w, `{"pageInfo":{"pages":0,"results":0},"results":[]}`)
				case "/api/v2":
					switch r.URL.Query().Get("cmd") {
					case "get_server_info":
						fmt.Fprint(w, `{"response":{"result":"success"}}`)
					case "get_history":
						if failure == "history" {
							http.Error(w, "history lookup failed", http.StatusServiceUnavailable)
							return
						}
						fmt.Fprint(w, `{"response":{"result":"success","data":{"data":[{"grandparent_rating_key":"100","user":"viewer","parent_media_index":1,"media_index":1,"percent_complete":100,"date":200},{"grandparent_rating_key":"100","user":"viewer","parent_media_index":2,"media_index":1,"percent_complete":100,"date":100}]}}}`)
					case "get_metadata":
						fmt.Fprint(w, `{"response":{"result":"success","data":{"media_type":"show","rating_key":"100","title":"Example Show","year":"2020","guids":["tvdb://10"]}}}`)
					default:
						http.Error(w, "unexpected command", http.StatusBadRequest)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			logger := zerolog.Nop()
			client := &Client{api: starrsonarr.New(starr.New("test-key", server.URL, time.Second)), logger: logger}
			o := NewOperations(client, logger)
			watchClient, err := tautulli.NewClient(server.URL, "test-key", logger)
			if err != nil {
				t.Fatal(err)
			}
			o.SetTautulliClient(watchClient)
			requestClient, err := overseerr.NewClient(server.URL, "test-key", logger)
			if err != nil {
				t.Fatal(err)
			}
			o.SetOverseerrClient(requestClient)
			series, err := o.GetAllSeries(context.Background())
			if failure != "" {
				if err == nil || series != nil {
					t.Fatal("failed enrichment must stop selection")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(series) != 1 {
				t.Fatalf("got %d series, want 1", len(series))
			}
			s := series[0]
			if s.EpisodeCount != 2 || s.WatchedEpisodes != 1 || s.WatchProgress != 50 || s.Watched {
				t.Fatalf("incorrect progress for current episodes: %+v", s)
			}
			if s.UserWatchData["viewer"].Progress != 50 || s.LastWatched.Unix() != 200 {
				t.Fatalf("incorrect user progress or last activity: %+v", s)
			}
		})
	}
}
