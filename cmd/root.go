package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/soup/radarr-cleanup/config"
	"github.com/soup/radarr-cleanup/filter"
	"github.com/soup/radarr-cleanup/radarr"
	"github.com/soup/radarr-cleanup/tautulli"
)

var (
	cfgFile        string
	cfg            *config.Config
	logger         zerolog.Logger
	radarrClient   *radarr.Client
	tautulliClient *tautulli.Client
	operations     *radarr.Operations

	// Command flags
	filterExpr    string
	preset        string
	dryRun        bool
	noConfirm     bool
	deleteFiles   bool
	ignoreWatched bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "radarr-cleanup",
	Short: "A tool to manage and clean up Radarr movies based on filters",
	Long: `radarr-cleanup is a CLI tool that allows you to search and delete movies
from your Radarr library based on various filter criteria including tags,
date added, and date imported.`,
	PersistentPreRunE: initializeApp,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "perform a dry run without making changes")

	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(testCmd)
}

// initializeApp initializes the configuration and clients
func initializeApp(cmd *cobra.Command, args []string) error {
	// Load configuration
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	logger = setupLogger(cfg.Logging)

	// Override dry-run from command line if specified
	if cmd.Flags().Changed("dry-run") {
		cfg.Safety.DryRun = dryRun
	}

	// Create Radarr client
	radarrClient, err = radarr.NewClient(cfg.Radarr.URL, cfg.Radarr.APIKey, logger)
	if err != nil {
		return fmt.Errorf("failed to create Radarr client: %w", err)
	}

	operations = radarr.NewOperations(radarrClient, logger)

	// Create Tautulli client if enabled
	if cfg.Tautulli.Enabled && cfg.Tautulli.WatchCheck.Enabled {
		tautulliClient, err = tautulli.NewClient(cfg.Tautulli.URL, cfg.Tautulli.APIKey, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create Tautulli client, continuing without watch status")
		} else {
			operations.SetTautulliClient(tautulliClient)
			operations.SetMinWatchPercent(cfg.Tautulli.WatchCheck.MinWatchPercent)
			logger.Info().Msg("Tautulli integration enabled")
		}
	}

	return nil
}

// setupLogger configures the zerolog logger
func setupLogger(cfg config.LoggingConfig) zerolog.Logger {
	// Set log level
	level := zerolog.InfoLevel
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}

	zerolog.SetGlobalLevel(level)

	// Configure output format
	if cfg.Format == "json" {
		return zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	// Console format
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		NoColor:    !cfg.Color,
	}

	return zerolog.New(output).With().Timestamp().Logger()
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List movies matching the filter criteria",
	Long:  `List all movies in your Radarr library that match the specified filter criteria.`,
	RunE:  runList,
}

func init() {
	listCmd.Flags().StringVarP(&filterExpr, "filter", "f", "", "filter expression")
	listCmd.Flags().StringVarP(&preset, "preset", "p", "", "use a preset filter from config")
}

func runList(cmd *cobra.Command, args []string) error {
	// Determine filter expression
	expr, err := getFilterExpression()
	if err != nil {
		return err
	}

	logger.Info().Str("filter", expr).Msg("Searching movies")

	// Parse filter
	filterFunc, err := filter.ParseAndCreateFilter(expr)
	if err != nil {
		return fmt.Errorf("invalid filter expression: %w", err)
	}

	// Search movies
	ctx := context.Background()
	movies, err := operations.SearchMovies(ctx, filterFunc)
	if err != nil {
		return err
	}

	// Display results
	if len(movies) == 0 {
		fmt.Println("No movies found matching the filter criteria.")
		return nil
	}

	fmt.Printf("\nFound %d movies:\n", len(movies))
	fmt.Println(strings.Repeat("-", 80))

	for _, movie := range movies {
		fmt.Printf("• %s (%d)", movie.Title, movie.Year)
		if movie.Watched {
			fmt.Printf(" [WATCHED]")
		}
		fmt.Println()
		if cfg.Safety.ShowDetails {
			if len(movie.TagNames) > 0 {
				fmt.Printf("  Tags: %s\n", strings.Join(movie.TagNames, ", "))
			}
			fmt.Printf("  Added: %s\n", movie.Added.Format("2006-01-02"))
			if !movie.FileImported.IsZero() {
				fmt.Printf("  Imported: %s\n", movie.FileImported.Format("2006-01-02"))
			}
			if movie.WatchCount > 0 {
				fmt.Printf("  Watch Count: %d", movie.WatchCount)
				if !movie.LastWatched.IsZero() {
					fmt.Printf(" (Last: %s)", movie.LastWatched.Format("2006-01-02"))
				}
				fmt.Println()
			}
		}
	}

	return nil
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete movies matching the filter criteria",
	Long:  `Delete movies from your Radarr library that match the specified filter criteria.`,
	RunE:  runDelete,
}

func init() {
	deleteCmd.Flags().StringVarP(&filterExpr, "filter", "f", "", "filter expression")
	deleteCmd.Flags().StringVarP(&preset, "preset", "p", "", "use a preset filter from config")
	deleteCmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteFiles, "delete-files", true, "also delete movie files from disk")
	deleteCmd.Flags().BoolVar(&ignoreWatched, "ignore-watched", false, "delete movies even if they have been watched")
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Determine filter expression
	expr, err := getFilterExpression()
	if err != nil {
		return err
	}

	logger.Info().Str("filter", expr).Msg("Searching movies to delete")

	// Parse filter
	filterFunc, err := filter.ParseAndCreateFilter(expr)
	if err != nil {
		return fmt.Errorf("invalid filter expression: %w", err)
	}

	// Search movies
	ctx := context.Background()
	movies, err := operations.SearchMovies(ctx, filterFunc)
	if err != nil {
		return err
	}

	// Check for watched movies if not ignoring
	if !ignoreWatched {
		var watchedCount int
		for _, movie := range movies {
			if movie.Watched {
				watchedCount++
			}
		}
		if watchedCount > 0 && cfg.Safety.ConfirmDelete && !noConfirm {
			fmt.Printf("\n⚠️  WARNING: %d of %d movies have been watched!\n", watchedCount, len(movies))
			fmt.Printf("Are you sure you want to continue? Use --ignore-watched to bypass this check. [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(strings.TrimSpace(response)) != "y" {
				logger.Info().Msg("Deletion cancelled due to watched movies")
				return nil
			}
		}
	}

	// Delete movies
	deleteOpts := radarr.DeleteOptions{
		DryRun:        cfg.Safety.DryRun,
		DeleteFiles:   deleteFiles,
		ConfirmDelete: cfg.Safety.ConfirmDelete && !noConfirm,
	}

	return operations.DeleteMovies(ctx, movies, deleteOpts)
}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to Radarr",
	Long:  `Test the connection to your Radarr instance and display basic information.`,
	RunE:  runTest,
}

func runTest(cmd *cobra.Command, args []string) error {
	fmt.Printf("Testing connection to Radarr at %s...\n", cfg.Radarr.URL)

	// Connection is already tested during client creation
	fmt.Println("✓ Connection successful!")

	// Get some basic stats
	ctx := context.Background()
	movies, err := radarrClient.GetAllMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed to get movies: %w", err)
	}

	tags, err := radarrClient.GetTags(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	fmt.Printf("\nRadarr Statistics:\n")
	fmt.Printf("- Total movies: %d\n", len(movies))
	fmt.Printf("- Total tags: %d\n", len(tags))

	if len(tags) > 0 {
		fmt.Printf("\nAvailable tags:\n")
		for _, tag := range tags {
			fmt.Printf("  • %s (ID: %d)\n", tag.Label, tag.ID)
		}
	}

	// Test Tautulli if configured
	if tautulliClient != nil {
		fmt.Printf("\nTesting connection to Tautulli at %s...\n", cfg.Tautulli.URL)
		fmt.Println("✓ Tautulli connection successful!")
		fmt.Printf("- Watch status checking: %s\n", boolToStatus(cfg.Tautulli.WatchCheck.Enabled))
		fmt.Printf("- Minimum watch percent: %.0f%%\n", cfg.Tautulli.WatchCheck.MinWatchPercent)
	} else {
		fmt.Println("\nTautulli integration: Disabled")
	}

	return nil
}

func boolToStatus(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

// getFilterExpression determines the filter expression to use
func getFilterExpression() (string, error) {
	// Priority: command line filter > preset > default
	if filterExpr != "" {
		return filterExpr, nil
	}

	if preset != "" {
		if presetFilter, ok := cfg.Filter.Presets[preset]; ok {
			return presetFilter.Expression, nil
		}
		return "", fmt.Errorf("preset '%s' not found in config", preset)
	}

	if cfg.Filter.DefaultExpression != "" {
		return cfg.Filter.DefaultExpression, nil
	}

	return "", fmt.Errorf("no filter expression specified")
}
