package cmd

import (
	"context"
	"fmt"
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
	RunE: runHardlink,
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
				sizeMB := float64(movie.MovieFile.Size) / 1024 / 1024
				if sizeMB > 1024 {
					fmt.Printf("Size: %.1f GB\n", sizeMB/1024)
				} else {
					fmt.Printf("Size: %.1f MB\n", sizeMB)
				}
			}
		}
		
		fmt.Printf("Hardlinks: %d (not hardlinked)\n", movie.HardlinkCount)
		
		// Show qBittorrent status
		if movie.IsSeeding {
			fmt.Printf("Status: ✓ Found in qBittorrent (actively seeding)\n\n")
			
			// Ask to re-import
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
					// Re-import the movie
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
		} else {
			fmt.Printf("Status: ✗ Not found in qBittorrent\n\n")
			
			// Ask to delete and re-search
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
					// Delete and trigger new search
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