package radarr

import (
	"fmt"
	"path/filepath"
	"strings"

	"golift.io/starr/radarr"
)

// ConsoleFormatter provides console output formatting for movies
type ConsoleFormatter struct{}

// NewConsoleFormatter creates a new console formatter
func NewConsoleFormatter() *ConsoleFormatter {
	return &ConsoleFormatter{}
}

// FormatMovieList formats a list of movies for console display
func (f *ConsoleFormatter) FormatMovieList(movies []MovieInfo, options FormatOptions) string {
	if len(movies) == 0 {
		return "No movies found"
	}

	var sb strings.Builder

	// Header
	sb.WriteString("\nMovie")
	if len(movies) != 1 {
		sb.WriteString("s")
	}
	fmt.Fprintf(&sb, " (%d):\n\n", len(movies))

	// Format each movie
	for i, movie := range movies {
		isLast := i == len(movies)-1
		f.formatMovie(&sb, movie, isLast, options)

		if !isLast {
			sb.WriteString("\u2502\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// FormatMoviesToDelete formats movies for deletion confirmation
func (f *ConsoleFormatter) FormatMoviesToDelete(movies []MovieInfo) string {
	if len(movies) == 0 {
		return ""
	}

	var sb strings.Builder
	var watchedCount int

	// Header
	sb.WriteString("\nMovie")
	if len(movies) != 1 {
		sb.WriteString("s")
	}
	fmt.Fprintf(&sb, " to be deleted (%d):\n\n", len(movies))

	// Format each movie
	for i, movie := range movies {
		isLast := i == len(movies)-1
		prefix := "\u251c"
		if isLast {
			prefix = "\u2570"
		}

		fmt.Fprintf(&sb, "%s\u2500\u2500 %s (%d)\n", prefix, movie.Title, movie.Year)

		// Track watch status for warning
		if movie.Watched {
			watchedCount++
		}

		indent := "\u2502   "
		if isLast {
			indent = "    "
		}

		// Tags
		if len(movie.TagNames) > 0 {
			fmt.Fprintf(&sb, "%sTags: %s\n", indent, strings.Join(movie.TagNames, ", "))
		}

		// File path
		if movie.HasFile {
			fmt.Fprintf(&sb, "%sFile: %s\n", indent, movie.Path)
		}

		// Dates
		var dateParts []string
		if !movie.Added.IsZero() {
			dateParts = append(dateParts, fmt.Sprintf("Available: %s", movie.Added.Format("2006-01-02")))
		}
		if !movie.FileImported.IsZero() && !movie.FileImported.Equal(movie.Added) {
			dateParts = append(dateParts, fmt.Sprintf("Imported: %s", movie.FileImported.Format("2006-01-02")))
		}
		if !movie.MonitoredSince.IsZero() && !movie.MonitoredSince.Equal(movie.Added) {
			dateParts = append(dateParts, fmt.Sprintf("Monitored: %s", movie.MonitoredSince.Format("2006-01-02")))
		}
		if len(dateParts) > 0 {
			fmt.Fprintf(&sb, "%s%s\n", indent, strings.Join(dateParts, " | "))
		}

		// Watch info
		if movie.WatchCount > 0 {
			watchInfo := fmt.Sprintf("Watched %dx", movie.WatchCount)
			if !movie.LastWatched.IsZero() {
				watchInfo += fmt.Sprintf(" (last: %s)", movie.LastWatched.Format("2006-01-02"))
			}
			fmt.Fprintf(&sb, "%s%s\n", indent, watchInfo)
		}

		// Request info
		if movie.IsRequested {
			requestInfo := fmt.Sprintf("Requested by: %s", movie.RequestedBy)
			if !movie.RequestDate.IsZero() {
				requestInfo += fmt.Sprintf(" on %s", movie.RequestDate.Format("2006-01-02"))
			}
			if movie.RequestStatus != "" {
				requestInfo += fmt.Sprintf(" (Status: %s)", movie.RequestStatus)
			}
			fmt.Fprintf(&sb, "%s%s\n", indent, requestInfo)
		}

		if !isLast {
			sb.WriteString("\u2502\n")
		}
	}

	sb.WriteString("\n")

	return sb.String()
}

// FormatUpgradeCandidates formats upgrade candidates for display
func (f *ConsoleFormatter) FormatUpgradeCandidates(candidates []UpgradeResult) string {
	if len(candidates) == 0 {
		return "No movies need upgrading"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "\nMovies that can be upgraded (%d):\n\n", len(candidates))

	for i, candidate := range candidates {
		isLast := i == len(candidates)-1
		prefix := "\u251c"
		if isLast {
			prefix = "\u2570"
		}

		fmt.Fprintf(&sb, "%s\u2500\u2500 %s (%d)\n", prefix, candidate.Movie.Title, candidate.Movie.Year)

		indent := "\u2502   "
		if isLast {
			indent = "    "
		}

		// Current formats and score
		if len(candidate.CurrentFormats) > 0 {
			fmt.Fprintf(&sb, "%sCurrent Formats: %v (Score: %d)\n",
				indent, candidate.CurrentFormats, candidate.CurrentFormatScore)
		} else {
			fmt.Fprintf(&sb, "%sCurrent Formats: None (Score: %d)\n",
				indent, candidate.CurrentFormatScore)
		}

		// Missing formats
		if len(candidate.MissingFormats) > 0 {
			fmt.Fprintf(&sb, "%sMissing Formats: %v\n", indent, candidate.MissingFormats)
		}

		// Status info
		var statusParts []string
		if !candidate.IsAvailable {
			statusParts = append(statusParts, "Not Released")
		}
		if candidate.NeedsMonitoring {
			statusParts = append(statusParts, "Not Monitored")
		}
		if len(statusParts) > 0 {
			fmt.Fprintf(&sb, "%sStatus: %v\n", indent, statusParts)
		}

		// File info
		if candidate.Movie.MovieFile != nil && candidate.Movie.MovieFile.Path != "" {
			fmt.Fprintf(&sb, "%sFile: %s\n", indent, candidate.Movie.MovieFile.Path)
		}

		if !isLast {
			sb.WriteString("\u2502\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// PrintImportableItems prints importable items in a formatted way
func (f *ConsoleFormatter) PrintImportableItems(items []*radarr.ManualImportOutput) {
	fmt.Printf("\nFound %d importable item(s):\n\n", len(items))

	for i, item := range items {
		isLast := i == len(items)-1
		prefix := "\u251C\u2500 "
		indent := "\u2502  "

		if isLast {
			prefix = "\u2514\u2500 "
			indent = "   "
		}

		// File name and size
		fmt.Printf("%s%s (%.2f MB)\n", prefix, filepath.Base(item.Path), float64(item.Size)/1024/1024)

		// Full path
		fmt.Printf("%sPath: %s\n", indent, item.Path)

		// Movie info
		if item.Movie != nil {
			fmt.Printf("%sMovie: %s (%d)\n", indent, item.Movie.Title, item.Movie.Year)
		}

		// Quality
		if item.Quality != nil && item.Quality.Quality != nil {
			fmt.Printf("%sQuality: %s\n", indent, item.Quality.Quality.Name)
		}

		// Rejections
		if len(item.Rejections) > 0 {
			fmt.Printf("%sRejections:\n", indent)
			for _, rejection := range item.Rejections {
				fmt.Printf("%s  - %s: %s\n", indent, rejection.Type, rejection.Reason)
			}
		}

		if !isLast {
			fmt.Println("\u2502")
		}
	}

	fmt.Println()
}

// formatMovie formats a single movie entry
func (f *ConsoleFormatter) formatMovie(sb *strings.Builder, movie MovieInfo, isLast bool, options FormatOptions) {
	prefix := "\u251c"
	if isLast {
		prefix = "\u2570"
	}

	fmt.Fprintf(sb, "%s\u2500\u2500 %s (%d)\n", prefix, movie.Title, movie.Year)

	indent := "\u2502   "
	if isLast {
		indent = "    "
	}

	// Basic info
	if len(movie.TagNames) > 0 && options.ShowDetails {
		fmt.Fprintf(sb, "%sTags: %s\n", indent, strings.Join(movie.TagNames, ", "))
	}

	if movie.HasFile && options.ShowDetails {
		fmt.Fprintf(sb, "%sFile: %s\n", indent, movie.Path)
		if movie.IsHardlinked {
			fmt.Fprintf(sb, "%sHardlinks: %d\n", indent, movie.HardlinkCount)
		}
	}

	// Watch info
	if options.ShowWatchInfo && movie.WatchCount > 0 {
		watchInfo := fmt.Sprintf("Watched %dx", movie.WatchCount)
		if !movie.LastWatched.IsZero() {
			watchInfo += fmt.Sprintf(" (last: %s)", movie.LastWatched.Format("2006-01-02"))
		}
		fmt.Fprintf(sb, "%s%s\n", indent, watchInfo)

		// Per-user watch data
		if len(movie.UserWatchData) > 0 {
			for username, userData := range movie.UserWatchData {
				if userData.Watched {
					userInfo := fmt.Sprintf("  - %s: %dx", username, userData.WatchCount)
					if userData.MaxProgress > 0 {
						userInfo += fmt.Sprintf(" (%.0f%%)", userData.MaxProgress)
					}
					fmt.Fprintf(sb, "%s%s\n", indent, userInfo)
				}
			}
		}
	}

	// Request info
	if options.ShowRequests && movie.IsRequested {
		requestInfo := fmt.Sprintf("Requested by: %s", movie.RequestedBy)
		if !movie.RequestDate.IsZero() {
			requestInfo += fmt.Sprintf(" on %s", movie.RequestDate.Format("2006-01-02"))
		}
		fmt.Fprintf(sb, "%s%s\n", indent, requestInfo)
	}

	// Ratings
	if options.ShowDetails && len(movie.Ratings) > 0 {
		var ratings []string
		for source, rating := range movie.Ratings {
			ratings = append(ratings, fmt.Sprintf("%s: %.1f", source, rating))
		}
		fmt.Fprintf(sb, "%sRatings: %s\n", indent, strings.Join(ratings, ", "))
	}
}

// FormatHardlinkResults formats hardlink scan results
func (f *ConsoleFormatter) FormatHardlinkResults(movies []MovieInfo) string {
	if len(movies) == 0 {
		return "All movies are properly hardlinked"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "\nNon-hardlinked movies (%d):\n\n", len(movies))

	// Group by status
	var inQBittorrent, notInQBittorrent []MovieInfo
	for _, movie := range movies {
		if movie.QBittorrentHash != "" {
			inQBittorrent = append(inQBittorrent, movie)
		} else {
			notInQBittorrent = append(notInQBittorrent, movie)
		}
	}

	// Format movies in qBittorrent
	if len(inQBittorrent) > 0 {
		sb.WriteString("Movies found in qBittorrent (can be re-imported):\n")
		for i, movie := range inQBittorrent {
			isLast := i == len(inQBittorrent)-1
			prefix := "\u251c"
			if isLast {
				prefix = "\u2570"
			}

			fmt.Fprintf(&sb, "%s\u2500\u2500 %s (%d)", prefix, movie.Title, movie.Year)
			if movie.IsSeeding {
				sb.WriteString(" [SEEDING]")
			}
			sb.WriteString("\n")

			if !isLast {
				sb.WriteString("\u2502\n")
			}
		}
		sb.WriteString("\n")
	}

	// Format movies not in qBittorrent
	if len(notInQBittorrent) > 0 {
		sb.WriteString("Movies not found in qBittorrent (need re-download):\n")
		for i, movie := range notInQBittorrent {
			isLast := i == len(notInQBittorrent)-1
			prefix := "\u251c"
			if isLast {
				prefix = "\u2570"
			}

			fmt.Fprintf(&sb, "%s\u2500\u2500 %s (%d)\n", prefix, movie.Title, movie.Year)

			if !isLast {
				sb.WriteString("\u2502\n")
			}
		}
	}

	return sb.String()
}
