package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/s0up4200/arrbiter/config"
	"github.com/s0up4200/arrbiter/filter"
	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/radarr"
	"github.com/s0up4200/arrbiter/tautulli"
)

var (
	cfgFile         string
	cfg             *config.Config
	logger          zerolog.Logger
	radarrClient    *radarr.Client
	tautulliClient  *tautulli.Client
	overseerrClient *overseerr.Client
	operations      *radarr.Operations

	// Command flags
	dryRun        bool
	noConfirm     bool
	deleteFiles   bool
	ignoreWatched bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "arrbiter",
	Short: "Your media library's arbiter of taste",
	Long: `Arrbiter is a CLI tool that intelligently manages your Radarr library
using advanced filter expressions. It integrates with Tautulli for watch tracking
and Overseerr for request management to make informed decisions about your media.`,
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

	// Create Overseerr client if enabled
	if cfg.Overseerr.Enabled {
		overseerrClient, err = overseerr.NewClient(cfg.Overseerr.URL, cfg.Overseerr.APIKey, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create Overseerr client, continuing without request data")
		} else {
			operations.SetOverseerrClient(overseerrClient)
			logger.Info().Msg("Overseerr integration enabled")
		}
	}

	return nil
}

// setupLogger configures the zerolog logger
func setupLogger(cfg config.LoggingConfig) zerolog.Logger {
	// Set log level
	level := zerolog.InfoLevel
	switch strings.ToLower(cfg.Level) {
	case "trace":
		level = zerolog.TraceLevel
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
	// No filter flags needed anymore
}

func runList(cmd *cobra.Command, args []string) error {
	// Check if any filters are defined
	if len(cfg.Filter) == 0 {
		fmt.Println("No filters defined in configuration.")
		return nil
	}

	logger.Info().Int("filter_count", len(cfg.Filter)).Msg("Processing filters")

	// Get all movies once
	ctx := context.Background()
	allMovies, err := operations.GetAllMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed to get movies: %w", err)
	}

	// Track which movies match which filters
	moviesByFilter := make(map[string][]radarr.MovieInfo)
	matchedMovies := make(map[int64]bool) // Track unique movies by ID

	// Process each filter
	for filterName, filterExpr := range cfg.Filter {
		logger.Debug().Str("filter", filterName).Str("expression", filterExpr).Msg("Processing filter")
		
		// Parse filter
		filterFunc, err := filter.ParseAndCreateFilter(filterExpr)
		if err != nil {
			logger.Error().Err(err).Str("filter", filterName).Msg("Invalid filter expression")
			continue
		}

		// Find matching movies
		for _, movie := range allMovies {
			if filterFunc(movie) {
				moviesByFilter[filterName] = append(moviesByFilter[filterName], movie)
				matchedMovies[movie.ID] = true
			}
		}
	}

	// Display results
	if len(matchedMovies) == 0 {
		fmt.Println("No movies found matching any filter criteria.")
		return nil
	}

	fmt.Printf("\nFound %d movies:\n", len(matchedMovies))
	fmt.Println(strings.Repeat("-", 80))

	// Display movies grouped by filter
	for filterName, movies := range moviesByFilter {
		if len(movies) == 0 {
			continue
		}
		
		fmt.Printf("\nFrom filter \"%s\":\n", filterName)
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
	deleteCmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteFiles, "delete-files", true, "also delete movie files from disk")
	deleteCmd.Flags().BoolVar(&ignoreWatched, "ignore-watched", false, "delete movies even if they have been watched")
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Check if any filters are defined
	if len(cfg.Filter) == 0 {
		fmt.Println("No filters defined in configuration.")
		return nil
	}

	logger.Info().Int("filter_count", len(cfg.Filter)).Msg("Processing filters for deletion")

	// Get all movies once
	ctx := context.Background()
	allMovies, err := operations.GetAllMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed to get movies: %w", err)
	}

	// Track which movies match which filters
	moviesByFilter := make(map[string][]radarr.MovieInfo)
	uniqueMovies := make(map[int64]radarr.MovieInfo) // Track unique movies by ID
	
	// Process each filter
	for filterName, filterExpr := range cfg.Filter {
		logger.Debug().Str("filter", filterName).Str("expression", filterExpr).Msg("Processing filter")
		
		// Parse filter
		filterFunc, err := filter.ParseAndCreateFilter(filterExpr)
		if err != nil {
			logger.Error().Err(err).Str("filter", filterName).Msg("Invalid filter expression")
			continue
		}

		// Find matching movies
		for _, movie := range allMovies {
			if filterFunc(movie) {
				moviesByFilter[filterName] = append(moviesByFilter[filterName], movie)
				uniqueMovies[movie.ID] = movie
			}
		}
	}

	// Convert unique movies to slice
	var moviesToDelete []radarr.MovieInfo
	for _, movie := range uniqueMovies {
		moviesToDelete = append(moviesToDelete, movie)
	}

	if len(moviesToDelete) == 0 {
		fmt.Println("No movies found matching any filter criteria.")
		return nil
	}

	// Display what will be deleted, grouped by filter
	fmt.Printf("\nFound %d movies to delete:\n", len(moviesToDelete))
	fmt.Println(strings.Repeat("-", 80))
	
	for filterName, movies := range moviesByFilter {
		if len(movies) == 0 {
			continue
		}
		
		fmt.Printf("\nFrom filter \"%s\":\n", filterName)
		for _, movie := range movies {
			fmt.Printf("• %s (%d)", movie.Title, movie.Year)
			if movie.Watched {
				fmt.Printf(" [WATCHED]")
			}
			fmt.Println()
		}
	}

	// Check for watched movies if not ignoring
	if !ignoreWatched {
		var watchedCount int
		for _, movie := range moviesToDelete {
			if movie.Watched {
				watchedCount++
			}
		}
		if watchedCount > 0 && cfg.Safety.ConfirmDelete && !noConfirm {
			fmt.Printf("\n⚠️  WARNING: %d of %d movies have been watched!\n", watchedCount, len(moviesToDelete))
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

	return operations.DeleteMovies(ctx, moviesToDelete, deleteOpts)
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

