package radarr

import (
	"testing"
	"time"

	"golift.io/starr/radarr"
)

func TestIsMovieAvailable(t *testing.T) {
	o := &Operations{}
	now := time.Now()
	pastDate := now.AddDate(0, -6, 0)  // 6 months ago
	futureDate := now.AddDate(0, 6, 0) // 6 months from now

	tests := []struct {
		name     string
		movie    *radarr.Movie
		expected bool
	}{
		{
			name: "Movie marked as available",
			movie: &radarr.Movie{
				IsAvailable: true,
			},
			expected: true,
		},
		{
			name: "Digital release in past",
			movie: &radarr.Movie{
				DigitalRelease: pastDate,
			},
			expected: true,
		},
		{
			name: "Digital release in future",
			movie: &radarr.Movie{
				DigitalRelease: futureDate,
			},
			expected: false,
		},
		{
			name: "Physical release in past",
			movie: &radarr.Movie{
				PhysicalRelease: pastDate,
			},
			expected: true,
		},
		{
			name: "In cinemas more than 4 months ago",
			movie: &radarr.Movie{
				InCinemas: now.AddDate(0, -5, 0),
			},
			expected: true,
		},
		{
			name: "In cinemas less than 4 months ago",
			movie: &radarr.Movie{
				InCinemas: now.AddDate(0, -2, 0),
			},
			expected: false,
		},
		{
			name: "No release dates set",
			movie: &radarr.Movie{
				IsAvailable: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := o.IsMovieAvailable(tt.movie)
			if result != tt.expected {
				t.Errorf("IsMovieAvailable() = %v, want %v", result, tt.expected)
			}
		})
	}
}