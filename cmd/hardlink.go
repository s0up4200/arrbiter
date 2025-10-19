package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	noConfirmHardlink bool
)

// hardlinkCmd represents the hardlink command
var hardlinkCmd = &cobra.Command{
	Use:   "hardlink",
	Short: "Manage non-hardlinked movies between Radarr and qBittorrent",
	Long: `Scan for movies that are not hardlinked and manage them appropriately.

This command helps ensure proper hardlinking between Radarr and qBittorrent by:
- Detecting movies that don't have hardlinks
- Re-importing movies that exist in qBittorrent to create hardlinks
- Optionally deleting and re-searching for movies not in qBittorrent`,
	PreRunE: initializeApp,
	RunE:    runHardlink,
}

func init() {
	rootCmd.AddCommand(hardlinkCmd)

	hardlinkCmd.Flags().BoolVar(&noConfirmHardlink, "no-confirm", false, "skip confirmation prompts")
}

func runHardlink(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if qBittorrent is configured
	if cfg.QBittorrent.URL == "" || cfg.QBittorrent.Username == "" {
		return fmt.Errorf("qBittorrent configuration missing. Please set qbittorrent.url and qbittorrent.username in config")
	}

	// Scan for non-hardlinked movies
	logger.Info().Msg("Scanning for non-hardlinked movies...")

	nonHardlinkedMovies, err := operations.ScanNonHardlinkedMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed to scan for non-hardlinked movies: %w", err)
	}

	if len(nonHardlinkedMovies) == 0 {
		fmt.Println("✓ All movies are properly hardlinked!")
		return nil
	}

	fmt.Printf("Found %d non-hardlinked movie", len(nonHardlinkedMovies))
	if len(nonHardlinkedMovies) != 1 {
		fmt.Printf("s")
	}
	fmt.Println(".")

	// Process each movie interactively
	var processedCount, reimportedCount, deletedCount, skippedCount int

	for i, movie := range nonHardlinkedMovies {
		// Display movie information
		fmt.Printf("[%d/%d] %s (%d)\n", i+1, len(nonHardlinkedMovies), movie.Title, movie.Year)
		fmt.Println(strings.Repeat("━", 50))

		if movie.MovieFile != nil && movie.MovieFile.Path != "" {
			fmt.Printf("Path: %s\n", movie.MovieFile.Path)
			if movie.MovieFile.Size > 0 {
				fmt.Printf("Size: %s\n", formatSize(movie.MovieFile.Size))
			}
		}

		fmt.Printf("Hardlinks: %d (not hardlinked)\n", movie.HardlinkCount)

		// Show qBittorrent status
		statusHandled := false

		if movie.IsSeeding {
			fmt.Printf("Status: ✓ Found in qBittorrent (actively seeding)\n\n")
			statusHandled = true

			if !dryRun {
				response := "n"
				if !noConfirmHardlink {
					fmt.Printf("→ Re-import from qBittorrent to create hardlink? [y/n/q]: ")
					fmt.Scanln(&response)
				} else {
					response = "y"
				}

				response = strings.ToLower(strings.TrimSpace(response))

				if response == "q" || response == "quit" {
					fmt.Printf("\nProcessing stopped by user.\n")
					break
				} else if response == "y" || response == "yes" {
					if err := operations.ReimportMovieFromQBittorrent(ctx, movie); err != nil {
						logger.Error().Err(err).Str("movie", movie.Title).Msg("Failed to re-import movie")
						fmt.Printf("✗ Failed to re-import: %v\n", err)
					} else {
						fmt.Printf("✓ Re-imported successfully\n")
						reimportedCount++
					}
				} else {
					fmt.Printf("⊘ Skipped\n")
					skippedCount++
				}
			} else {
				fmt.Printf("[DRY RUN] Would re-import from qBittorrent\n")
			}
		}

		if !statusHandled && len(movie.AlternateTorrents) > 0 {
			fmt.Printf("Status: △ Alternate torrents available in qBittorrent\n")
			statusHandled = true

			for idx, match := range movie.AlternateTorrents {
				if match == nil || match.Torrent == nil {
					continue
				}

				torrent := match.Torrent
				sizeLabel := formatSize(torrent.Size)
				diffLabel := ""

				if movie.MovieFile != nil && movie.MovieFile.Size > 0 && torrent.Size > 0 {
					percent := float64(match.SizeDifference) / float64(movie.MovieFile.Size) * 100
					switch {
					case percent > 1 || percent < -1:
						diffLabel = fmt.Sprintf(" (%+.1f%% vs library)", percent)
					case percent == 0:
						diffLabel = " (same size)"
					default:
						diffLabel = " (~same size)"
					}
				}

				statusParts := []string{
					fmt.Sprintf("Score %.0f%%", match.Score*100),
					fmt.Sprintf("Progress %.1f%%", torrent.Progress*100),
				}

				if torrent.IsSeeding {
					statusParts = append(statusParts, "Seeding")
				} else if torrent.Progress >= 1.0 {
					statusParts = append(statusParts, "Complete")
				}

				if match.YearMatched {
					statusParts = append(statusParts, "Year match")
				}

				fmt.Printf("  [%d] %s\n", idx+1, torrent.Name)
				fmt.Printf("      %s | Size %s%s\n", strings.Join(statusParts, " | "), sizeLabel, diffLabel)
			}
			fmt.Println()

			if !dryRun {
				response := "n"
				if !noConfirmHardlink {
					fmt.Printf("→ Choose alternate torrent to re-import [1-%d/n/q]: ", len(movie.AlternateTorrents))
					fmt.Scanln(&response)
				} else {
					response = "1"
				}

				response = strings.ToLower(strings.TrimSpace(response))

				if response == "q" || response == "quit" {
					fmt.Printf("\nProcessing stopped by user.\n")
					break
				} else if response == "n" || response == "" || response == "no" {
					fmt.Printf("⊘ Skipped\n")
					skippedCount++
				} else {
					index, err := strconv.Atoi(response)
					if err != nil || index < 1 || index > len(movie.AlternateTorrents) {
						fmt.Printf("⊘ Invalid selection (skipped)\n")
						skippedCount++
					} else {
						match := movie.AlternateTorrents[index-1]
						if err := operations.ReimportMovieFromTorrentMatch(ctx, movie, match); err != nil {
							logger.Error().Err(err).Str("movie", movie.Title).Msg("Failed to re-import movie from alternate torrent")
							fmt.Printf("✗ Failed to re-import: %v\n", err)
						} else {
							fmt.Printf("✓ Re-imported using \"%s\"\n", match.Torrent.Name)
							reimportedCount++
						}
					}
				}
			} else {
				if len(movie.AlternateTorrents) > 0 && movie.AlternateTorrents[0] != nil && movie.AlternateTorrents[0].Torrent != nil {
					fmt.Printf("[DRY RUN] Would re-import using alternate torrent \"%s\"\n", movie.AlternateTorrents[0].Torrent.Name)
				} else {
					fmt.Printf("[DRY RUN] Would re-import using highest ranked alternate torrent\n")
				}
			}
		}

		if !statusHandled {
			fmt.Printf("Status: ✗ Not found in qBittorrent\n\n")

			if !dryRun {
				response := "n"
				if !noConfirmHardlink {
					fmt.Printf("→ Delete file and search for new version? [y/n/q]: ")
					fmt.Scanln(&response)
				} else {
					response = "y"
				}

				response = strings.ToLower(strings.TrimSpace(response))

				if response == "q" || response == "quit" {
					fmt.Printf("\nProcessing stopped by user.\n")
					break
				} else if response == "y" || response == "yes" {
					if err := operations.DeleteAndResearchMovie(ctx, movie); err != nil {
						logger.Error().Err(err).Str("movie", movie.Title).Msg("Failed to delete and re-search movie")
						fmt.Printf("✗ Failed to delete and re-search: %v\n", err)
					} else {
						fmt.Printf("✓ Deleted and searching for new version\n")
						deletedCount++
					}
				} else {
					fmt.Printf("⊘ Skipped\n")
					skippedCount++
				}
			} else {
				fmt.Printf("[DRY RUN] Would delete and re-search\n")
			}
		}

		processedCount++
		fmt.Println() // Empty line between movies
	}

	// Show summary
	fmt.Println("\nSummary:")
	fmt.Println(strings.Repeat("━", 50))
	fmt.Printf("Processed: %d movie", processedCount)
	if processedCount != 1 {
		fmt.Printf("s")
	}
	fmt.Println()

	if !dryRun {
		if reimportedCount > 0 {
			fmt.Printf("- Re-imported: %d\n", reimportedCount)
		}
		if deletedCount > 0 {
			fmt.Printf("- Deleted: %d\n", deletedCount)
		}
		if skippedCount > 0 {
			fmt.Printf("- Skipped: %d\n", skippedCount)
		}

		remaining := len(nonHardlinkedMovies) - processedCount
		if remaining > 0 {
			fmt.Printf("Remaining: %d movie", remaining)
			if remaining != 1 {
				fmt.Printf("s")
			}
			fmt.Println()
		}
	}

	return nil
}

func formatSize(bytes int64) string {
	if bytes <= 0 {
		return "unknown"
	}

	sizeMB := float64(bytes) / 1024.0 / 1024.0
	if sizeMB > 1024 {
		return fmt.Sprintf("%.1f GB", sizeMB/1024.0)
	}
	return fmt.Sprintf("%.1f MB", sizeMB)
}
