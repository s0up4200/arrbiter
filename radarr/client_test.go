package radarr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

type failedEnricher struct{ err error }

func (e failedEnricher) EnrichMovies(context.Context, []MovieInfo) error { return e.err }

func TestMovieEnrichmentFailureStopsSelection(t *testing.T) {
	lookupErr := errors.New("history lookup failed")
	api := &mockRadarrAPI{movies: []*radarr.Movie{{ID: 1, MovieFile: &radarr.MovieFile{DateAdded: time.Now()}}}}
	o := NewOperations(NewClientWithAPI(api, zerolog.Nop()), zerolog.Nop())
	o.enrichers = []MovieEnricher{failedEnricher{lookupErr}}
	if movies, err := o.GetAllMovies(context.Background()); !errors.Is(err, lookupErr) || movies != nil {
		t.Fatalf("GetAllMovies = %v, %v; want lookup error and no movies", movies, err)
	}
	if movies, err := o.SearchMovies(context.Background(), func(MovieInfo) bool {
		t.Fatal("must not evaluate filters after a failed lookup")
		return true
	}); !errors.Is(err, lookupErr) || movies != nil {
		t.Fatalf("SearchMovies = %v, %v; want lookup error and no movies", movies, err)
	}
}

// mockRadarrAPI implements RadarrAPI for testing
type mockRadarrAPI struct {
	movies          []*radarr.Movie
	tags            []*starr.Tag
	customFormats   []*radarr.CustomFormatOutput
	movieFiles      map[int64]*radarr.MovieFile
	deleteFileFlags []bool
	deleteErrors    map[int64]error
	deleteMu        sync.Mutex

	// Track calls for verification
	getMovieCalls int
	getTagsCalls  int
}

func (m *mockRadarrAPI) GetMovieContext(ctx context.Context, params *radarr.GetMovie) ([]*radarr.Movie, error) {
	m.getMovieCalls++
	return m.movies, nil
}

func (m *mockRadarrAPI) GetMovieByIDContext(ctx context.Context, movieID int64) (*radarr.Movie, error) {
	for _, movie := range m.movies {
		if movie.ID == movieID {
			return movie, nil
		}
	}
	return nil, nil
}

func (m *mockRadarrAPI) UpdateMovieContext(ctx context.Context, movieID int64, movie *radarr.Movie, moveFiles bool) (*radarr.Movie, error) {
	return movie, nil
}

func (m *mockRadarrAPI) DeleteMovieContext(ctx context.Context, movieID int64, deleteFiles, addImportExclusion bool) error {
	m.deleteMu.Lock()
	defer m.deleteMu.Unlock()
	m.deleteFileFlags = append(m.deleteFileFlags, deleteFiles)
	return m.deleteErrors[movieID]
}

func (m *mockRadarrAPI) GetMovieFileByIDContext(ctx context.Context, fileID int64) (*radarr.MovieFile, error) {
	if file, ok := m.movieFiles[fileID]; ok {
		return file, nil
	}
	return nil, nil
}

func (m *mockRadarrAPI) DeleteMovieFilesContext(ctx context.Context, movieFileIDs ...int64) error {
	return nil
}

func (m *mockRadarrAPI) GetTagsContext(ctx context.Context) ([]*starr.Tag, error) {
	m.getTagsCalls++
	return m.tags, nil
}

func (m *mockRadarrAPI) GetCustomFormatsContext(ctx context.Context) ([]*radarr.CustomFormatOutput, error) {
	return m.customFormats, nil
}

func (m *mockRadarrAPI) SendCommandContext(ctx context.Context, cmd *radarr.CommandRequest) (*radarr.CommandResponse, error) {
	return &radarr.CommandResponse{
		ID:     1,
		Name:   cmd.Name,
		Status: "completed",
	}, nil
}

func (m *mockRadarrAPI) ManualImportContext(ctx context.Context, params *radarr.ManualImportParams) (*radarr.ManualImportOutput, error) {
	return nil, nil
}

func (m *mockRadarrAPI) ManualImportReprocessContext(ctx context.Context, item *radarr.ManualImportInput) error {
	return nil
}

func (m *mockRadarrAPI) Ping() error {
	return nil
}

func TestClient_GetTags_Caching(t *testing.T) {
	// Setup
	mockAPI := &mockRadarrAPI{
		tags: []*starr.Tag{
			{ID: 1, Label: "tag1"},
			{ID: 2, Label: "tag2"},
		},
	}

	logger := zerolog.New(nil).Level(zerolog.Disabled)
	client := NewClientWithAPI(mockAPI, logger)
	client.cacheTTL = 1 * time.Second

	ctx := context.Background()

	// First call should hit the API
	tags1, err := client.GetTags(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags1) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags1))
	}
	if mockAPI.getTagsCalls != 1 {
		t.Errorf("expected 1 API call, got %d", mockAPI.getTagsCalls)
	}

	// Second call should use cache
	tags2, err := client.GetTags(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags2) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags2))
	}
	if mockAPI.getTagsCalls != 1 {
		t.Errorf("expected 1 API call (cached), got %d", mockAPI.getTagsCalls)
	}

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond)

	// Third call should hit the API again
	tags3, err := client.GetTags(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags3) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags3))
	}
	if mockAPI.getTagsCalls != 2 {
		t.Errorf("expected 2 API calls (cache expired), got %d", mockAPI.getTagsCalls)
	}
}

func TestClient_GetMovieInfo(t *testing.T) {
	testTime := time.Now()

	tests := []struct {
		name     string
		movie    *radarr.Movie
		tags     []*starr.Tag
		expected MovieInfo
	}{
		{
			name: "Basic movie with tags",
			movie: &radarr.Movie{
				ID:      1,
				Title:   "Test Movie",
				Year:    2023,
				TmdbID:  12345,
				ImdbID:  "tt1234567",
				Path:    "/movies/test",
				Tags:    []int{1, 2},
				Added:   testTime,
				HasFile: true,
				MovieFile: &radarr.MovieFile{
					ID:        10,
					Path:      "/movies/test/test.mkv",
					DateAdded: testTime,
				},
				Popularity: 8.5,
			},
			tags: []*starr.Tag{
				{ID: 1, Label: "action"},
				{ID: 2, Label: "adventure"},
			},
			expected: MovieInfo{
				ID:             1,
				Title:          "Test Movie",
				Year:           2023,
				TMDBID:         12345,
				IMDBID:         "tt1234567",
				Path:           "/movies/test",
				Tags:           []int{1, 2},
				TagNames:       []string{"action", "adventure"},
				Added:          testTime,
				MonitoredSince: testTime,
				HasFile:        true,
				FileImported:   testTime,
				Popularity:     8.5,
			},
		},
		{
			name: "Movie with ratings",
			movie: &radarr.Movie{
				ID:    2,
				Title: "Rated Movie",
				Year:  2023,
				Ratings: starr.OpenRatings{
					"imdb": starr.Ratings{Value: 7.5},
					"tmdb": starr.Ratings{Value: 8.0},
				},
			},
			tags: []*starr.Tag{},
			expected: MovieInfo{
				ID:             2,
				Title:          "Rated Movie",
				Year:           2023,
				TagNames:       []string{},
				MonitoredSince: time.Time{},
				Ratings: map[string]float64{
					"imdb": 7.5,
					"tmdb": 8.0,
				},
			},
		},
		{
			name: "Separates monitored and availability dates",
			movie: &radarr.Movie{
				ID:      3,
				Title:   "Delayed Grab",
				Year:    2022,
				Added:   testTime.Add(-48 * time.Hour),
				HasFile: true,
				MovieFile: &radarr.MovieFile{
					ID:        11,
					DateAdded: testTime,
				},
			},
			tags: []*starr.Tag{},
			expected: MovieInfo{
				ID:             3,
				Title:          "Delayed Grab",
				Year:           2022,
				Added:          testTime,
				MonitoredSince: testTime.Add(-48 * time.Hour),
				HasFile:        true,
				FileImported:   testTime,
				TagNames:       []string{},
				Ratings:        map[string]float64{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.New(nil).Level(zerolog.Disabled)
			client := NewClientWithAPI(&mockRadarrAPI{}, logger)

			result := client.GetMovieInfo(tt.movie, tt.tags)

			// Check basic fields
			if result.ID != tt.expected.ID {
				t.Errorf("ID mismatch: got %d, want %d", result.ID, tt.expected.ID)
			}
			if result.Title != tt.expected.Title {
				t.Errorf("Title mismatch: got %s, want %s", result.Title, tt.expected.Title)
			}
			if len(result.TagNames) != len(tt.expected.TagNames) {
				t.Errorf("TagNames length mismatch: got %d, want %d", len(result.TagNames), len(tt.expected.TagNames))
			}

			if !result.Added.Equal(tt.expected.Added) {
				t.Errorf("Added mismatch: got %v, want %v", result.Added, tt.expected.Added)
			}
			if !result.MonitoredSince.Equal(tt.expected.MonitoredSince) {
				t.Errorf("MonitoredSince mismatch: got %v, want %v", result.MonitoredSince, tt.expected.MonitoredSince)
			}
			if !result.FileImported.Equal(tt.expected.FileImported) {
				t.Errorf("FileImported mismatch: got %v, want %v", result.FileImported, tt.expected.FileImported)
			}

			// Check ratings
			if len(result.Ratings) != len(tt.expected.Ratings) {
				t.Errorf("Ratings length mismatch: got %d, want %d", len(result.Ratings), len(tt.expected.Ratings))
			}
			for source, rating := range tt.expected.Ratings {
				if result.Ratings[source] != rating {
					t.Errorf("Rating mismatch for %s: got %f, want %f", source, result.Ratings[source], rating)
				}
			}
		})
	}
}

func TestBatchDeleteMovies(t *testing.T) {
	mockAPI := &mockRadarrAPI{}
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	client := NewClientWithAPI(mockAPI, logger)

	movies := []MovieInfo{
		{ID: 1, Title: "Movie 1", MovieFile: &radarr.MovieFile{Size: 1 << 30}},
		{ID: 2, Title: "Movie 2", MovieFile: &radarr.MovieFile{Size: 2 << 30}},
		{ID: 3, Title: "Movie 3"},
	}

	ctx := context.Background()
	result := client.BatchDeleteMovies(ctx, movies)

	if result.Requested != 3 {
		t.Errorf("expected 3 requested deletions, got %d", result.Requested)
	}
	if len(result.Successful) != 3 {
		t.Errorf("expected 3 successful deletions, got %d", len(result.Successful))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed deletions, got %d", len(result.Failed))
	}
	if result.DeletedBytes != 3<<30 {
		t.Errorf("expected 3 GiB deleted, got %d bytes", result.DeletedBytes)
	}
	if len(mockAPI.deleteFileFlags) != 3 {
		t.Fatalf("expected deleteFiles flag captured 3 times, got %d", len(mockAPI.deleteFileFlags))
	}
	for _, flag := range mockAPI.deleteFileFlags {
		if !flag {
			t.Error("expected deleteFiles flag to always be true")
		}
	}
}

func TestDeleteMoviesSummary(t *testing.T) {
	movies := []MovieInfo{
		{ID: 1, Title: "First Movie", Year: 2020, MovieFile: &radarr.MovieFile{Size: 3 << 29}},
		{ID: 2, Title: "Second Movie", Year: 2021, MovieFile: &radarr.MovieFile{Size: 2 << 30}},
		{ID: 3, Title: "Unknown Size", Year: 2022},
	}
	preview := NewConsoleFormatter().FormatMoviesToDelete(movies)
	if !strings.Contains(preview, "Total file size: 3.50 GiB") {
		t.Fatalf("preview omits the candidate size: %s", preview)
	}

	for _, tc := range []struct {
		name        string
		fail        bool
		dryRun      bool
		deleted     int
		failed      int
		deletedSize string
	}{
		{name: "success", deleted: 3, deletedSize: "3.50 GiB"},
		{name: "partial failure", fail: true, deleted: 2, failed: 1, deletedSize: "1.50 GiB"},
		{name: "dry run", dryRun: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI := &mockRadarrAPI{}
			if tc.fail {
				mockAPI.deleteErrors = map[int64]error{2: errors.New("delete refused")}
			}
			var logs bytes.Buffer
			logger := zerolog.New(zerolog.SyncWriter(&logs))
			operations := NewOperations(NewClientWithAPI(mockAPI, logger), logger)
			err := operations.DeleteMovies(context.Background(), movies, DeleteOptions{DryRun: tc.dryRun})
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected deletion error: %v", err)
			}
			if tc.dryRun {
				if len(mockAPI.deleteFileFlags) != 0 || strings.Contains(logs.String(), "Deletion complete") {
					t.Fatalf("dry run attempted deletion or reported success: %s", logs.String())
				}
				return
			}

			var last map[string]any
			successes := make(map[string]any)
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var event map[string]any
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				if event["message"] == "Successfully deleted movie" {
					title, ok := event["title"].(string)
					if !ok || event["movie_id"] != nil {
						t.Fatalf("success log must identify the movie by title: %s", line)
					}
					successes[title] = event["year"]
				}
				last = event
			}
			if len(successes) != tc.deleted {
				t.Fatalf("expected %d success logs, got %d", tc.deleted, len(successes))
			}
			for _, movie := range movies {
				if tc.fail && movie.ID == 2 {
					if _, ok := successes[movie.Title]; ok {
						t.Error("failed deletion was logged as successful")
					}
					continue
				}
				if successes[movie.Title] != float64(movie.Year) {
					t.Errorf("missing title or year for %s: %s", movie.Title, logs.String())
				}
			}
			if last["message"] != "Deletion complete" || last["deleted"] != float64(tc.deleted) ||
				last["failed"] != float64(tc.failed) || last["deleted_file_size"] != tc.deletedSize {
				t.Errorf("unexpected final summary: %v", last)
			}
		})
	}
}
