package cmd

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/s0up4200/arrbiter/radarr"
	"github.com/spf13/cobra"
)

var (
	unattendedCount int
	matchMode       string
	noMonitor       bool
)

// upgradeCmd represents the upgrade command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Find and upgrade movies missing custom formats",
	Long: `Scan for movies that don't have the configured custom formats and trigger upgrade searches.

This command helps ensure your library meets quality standards by:
- Detecting movies that don't match configured custom formats
- Allowing interactive or unattended upgrade of multiple movies
- Optionally monitoring upgraded movies for automatic downloads`,
	PreRunE: initializeApp,
	RunE:    runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)

	upgradeCmd.Flags().IntVar(&unattendedCount, "unattended", 0, "run in unattended mode, upgrading N movies")
	upgradeCmd.Flags().StringVar(&matchMode, "match", "", "override match mode (any/all)")
	upgradeCmd.Flags().BoolVar(&noMonitor, "no-monitor", false, "don't monitor movies after upgrade search")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if custom formats are configured
	if len(cfg.Upgrade.CustomFormats) == 0 {
		return fmt.Errorf("no custom formats configured. Please set upgrade.custom_formats in config")
	}

	// Override match mode if provided
	effectiveMatchMode := cfg.Upgrade.MatchMode
	if matchMode != "" {
		if matchMode != "any" && matchMode != "all" {
			return fmt.Errorf("invalid match mode: %s (must be 'any' or 'all')", matchMode)
		}
		effectiveMatchMode = matchMode
	}

	// Create upgrade options
	opts := radarr.UpgradeOptions{
		TargetCustomFormats: cfg.Upgrade.CustomFormats,
		CheckAvailability:   true,
		DryRun:              false, // We'll handle dry-run ourselves
	}

	// Scan for movies missing custom formats
	logger.Info().
		Strs("custom_formats", cfg.Upgrade.CustomFormats).
		Str("match_mode", effectiveMatchMode).
		Msg("Scanning for movies missing custom formats...")

	upgradeResults, err := operations.ScanMoviesForUpgrade(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to scan for movies needing upgrade: %w", err)
	}

	// Filter based on match mode
	var filteredResults []radarr.UpgradeResult
	for _, result := range upgradeResults {
		if effectiveMatchMode == "all" && len(result.MissingFormats) == len(cfg.Upgrade.CustomFormats) {
			// Movie is missing ALL custom formats
			filteredResults = append(filteredResults, result)
		} else if effectiveMatchMode == "any" && len(result.MissingFormats) > 0 {
			// Movie is missing ANY custom format
			filteredResults = append(filteredResults, result)
		}
	}

	if len(filteredResults) == 0 {
		fmt.Println("✓ All movies have the required custom formats!")
		return nil
	}

	// Display the list of movies
	movieText := "movie"
	if len(filteredResults) != 1 {
		movieText = "movies"
	}
	fmt.Printf("Found %d %s missing custom formats:\n\n", len(filteredResults), movieText)

	fmt.Println(strings.Repeat("━", 85))
	fmt.Printf("%-4s %-50s %-15s %s\n", "#", "MOVIE", "YEAR", "CURRENT FORMATS")
	fmt.Println(strings.Repeat("━", 85))

	for i, result := range filteredResults {
		// Build current custom formats string
		currentFormats := "None"
		if len(result.CurrentFormats) > 0 {
			currentFormats = strings.Join(result.CurrentFormats, ", ")
		}

		// Truncate title if too long
		title := result.Movie.Title
		if len(title) > 48 {
			title = title[:45] + "..."
		}

		fmt.Printf("%-4d %-50s %-15d %s\n", i+1, title, result.Movie.Year, currentFormats)
	}
	fmt.Println(strings.Repeat("━", 85))

	// Determine how many movies to upgrade
	var upgradeCount int
	var selectedResults []radarr.UpgradeResult

	if unattendedCount > 0 {
		// Unattended mode
		upgradeCount = min(unattendedCount, len(filteredResults))
		movieText := "movie"
		if upgradeCount != 1 {
			movieText = "movies"
		}
		fmt.Printf("\n[UNATTENDED MODE] Upgrading %d %s\n", upgradeCount, movieText)
	} else {
		// Interactive mode
		fmt.Printf("\nEnter movie numbers to upgrade (comma-separated, e.g. 1,3,5) or 'all' for all [Enter to cancel]: ")

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			// No input (Ctrl+D or similar)
			fmt.Println("No movies selected for upgrade.")
			return nil
		}
		input := scanner.Text()

		input = strings.TrimSpace(input)

		if input == "" {
			fmt.Println("No movies selected for upgrade.")
			return nil
		}

		var selectedIndices []int

		if strings.ToLower(input) == "all" {
			// Select all movies
			for i := range filteredResults {
				selectedIndices = append(selectedIndices, i)
			}
		} else {
			// Parse comma-separated numbers
			parts := strings.Split(input, ",")
			seen := make(map[int]bool)

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				num, err := strconv.Atoi(part)
				if err != nil {
					return fmt.Errorf("invalid number '%s': must be a positive integer", part)
				}

				if num < 1 || num > len(filteredResults) {
					return fmt.Errorf("invalid movie number %d: must be between 1 and %d", num, len(filteredResults))
				}

				// Convert to 0-based index and check for duplicates
				idx := num - 1
				if !seen[idx] {
					selectedIndices = append(selectedIndices, idx)
					seen[idx] = true
				}
			}

			if len(selectedIndices) == 0 {
				fmt.Println("No valid movies selected for upgrade.")
				return nil
			}
		}

		// Build selected results from indices
		for _, idx := range selectedIndices {
			selectedResults = append(selectedResults, filteredResults[idx])
		}
		upgradeCount = len(selectedResults)
	}

	// For unattended mode, keep the original random selection logic
	if unattendedCount > 0 {
		selectedResults = nil
		if upgradeCount == len(filteredResults) {
			selectedResults = filteredResults
		} else {
			// Randomly select movies
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			indices := rng.Perm(len(filteredResults))[:upgradeCount]

			for _, idx := range indices {
				selectedResults = append(selectedResults, filteredResults[idx])
			}
		}
	}

	// Override monitor setting if flag provided
	shouldMonitor := cfg.Upgrade.AutoMonitor
	if noMonitor {
		shouldMonitor = false
	}

	// Trigger upgrade searches
	movieText = "movie"
	if len(selectedResults) != 1 {
		movieText = "movies"
	}
	fmt.Printf("\nTriggering upgrade searches for %d %s...\n", len(selectedResults), movieText)

	if !dryRun {
		var successCount int
		var monitoringFailures int
		var searchFailures int
		var movieIDs []int64

		// Enable monitoring if needed
		if shouldMonitor {
			for _, result := range selectedResults {
				if result.NeedsMonitoring {
					fmt.Printf("→ Enabling monitoring for %s (%d)... ", result.Movie.Title, result.Movie.Year)
					err := operations.MonitorMovie(ctx, result.Movie.ID)
					if err != nil {
						logger.Error().Err(err).Str("movie", result.Movie.Title).Msg("Failed to enable monitoring")
						fmt.Printf("✗ Failed: %v\n", err)
						monitoringFailures++
					} else {
						fmt.Printf("✓ Enabled\n")
					}
				}
			}
		}

		// Collect movie IDs for batch search
		for _, result := range selectedResults {
			movieIDs = append(movieIDs, result.Movie.ID)
		}

		// Trigger searches in batches
		batchSize := 10
		for i := 0; i < len(movieIDs); i += batchSize {
			end := min(i+batchSize, len(movieIDs))

			batch := movieIDs[i:end]
			fmt.Printf("→ Searching batch of %d movies... ", len(batch))

			err := operations.TriggerUpgradeSearch(ctx, batch)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to trigger upgrade search")
				fmt.Printf("✗ Failed: %v\n", err)
				searchFailures += len(batch)
			} else {
				fmt.Printf("✓ Search triggered\n")
				successCount += len(batch)
			}

			// Add a small delay between batches
			if end < len(movieIDs) {
				time.Sleep(2 * time.Second)
			}
		}

		// Summary
		movieText = "movie"
		if successCount != 1 {
			movieText = "movies"
		}
		fmt.Printf("\n✓ Successfully triggered searches for %d %s\n", successCount, movieText)

		if searchFailures > 0 {
			movieText = "movie"
			if searchFailures != 1 {
				movieText = "movies"
			}
			fmt.Printf("✗ Failed to trigger searches for %d %s\n", searchFailures, movieText)
		}

		if monitoringFailures > 0 {
			movieText = "movie"
			if monitoringFailures != 1 {
				movieText = "movies"
			}
			fmt.Printf("✗ Failed to enable monitoring for %d %s\n", monitoringFailures, movieText)
		}
	} else {
		fmt.Println("[DRY RUN] Would trigger upgrade searches for:")
		for _, result := range selectedResults {
			fmt.Printf("  - %s (%d)", result.Movie.Title, result.Movie.Year)
			if shouldMonitor && result.NeedsMonitoring {
				fmt.Printf(" [would enable monitoring]")
			}
			fmt.Println()
		}
	}

	return nil
}
