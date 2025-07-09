package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	starr_radarr "golift.io/starr/radarr"

	"github.com/s0up4200/arrbiter/config"
	"github.com/s0up4200/arrbiter/filter"
	"github.com/s0up4200/arrbiter/overseerr"
	"github.com/s0up4200/arrbiter/qbittorrent"
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
	rootCmd.AddCommand(importCmd)
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
	logger.Info().Msg("Radarr integration enabled")

	operations = radarr.NewOperations(radarrClient, logger)

	// Create Tautulli client if URL and API key are provided
	if cfg.Tautulli.URL != "" && cfg.Tautulli.APIKey != "" {
		tautulliClient, err = tautulli.NewClient(cfg.Tautulli.URL, cfg.Tautulli.APIKey, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create Tautulli client, continuing without watch status")
		} else {
			operations.SetTautulliClient(tautulliClient)
			operations.SetMinWatchPercent(cfg.Tautulli.MinWatchPercent)
			logger.Info().Msg("Tautulli integration enabled")
		}
	}

	// Create Overseerr client if URL and API key are provided
	if cfg.Overseerr.URL != "" && cfg.Overseerr.APIKey != "" {
		overseerrClient, err = overseerr.NewClient(cfg.Overseerr.URL, cfg.Overseerr.APIKey, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create Overseerr client, continuing without request data")
		} else {
			operations.SetOverseerrClient(overseerrClient)
			logger.Info().Msg("Overseerr integration enabled")
		}
	}

	// Create qBittorrent client if URL and username are provided
	if cfg.QBittorrent.URL != "" && cfg.QBittorrent.Username != "" {
		qbittorrentClient, err := qbittorrent.NewClient(cfg.QBittorrent.URL, cfg.QBittorrent.Username, cfg.QBittorrent.Password, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create qBittorrent client, continuing without torrent integration")
		} else {
			operations.SetQBittorrentClient(qbittorrentClient)
			logger.Info().Msg("qBittorrent integration enabled")
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

	// Always use console writer for CLI
	// Auto-detect color support if not explicitly disabled
	noColor := !cfg.Color
	if cfg.Color && !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		// Disable color if not in a terminal (unless explicitly enabled in config)
		noColor = true
	}

	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    noColor,
		TimeFormat: "15:04:05",
	}

	return zerolog.New(output).With().Timestamp().Logger()
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List movies matching the filter criteria",
	Long:  `List all movies in your Radarr library that match the specified filter criteria.`,
	PreRunE: initializeApp,
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

	fmt.Printf("\nFound %d movie", len(matchedMovies))
	if len(matchedMovies) != 1 {
		fmt.Printf("s")
	}
	fmt.Println()
	fmt.Println()

	// Display movies grouped by filter
	for filterName, movies := range moviesByFilter {
		if len(movies) == 0 {
			continue
		}
		
		fmt.Printf("\u256d\u2500 Filter: %s (%d match", filterName, len(movies))
		if len(movies) != 1 {
			fmt.Printf("es")
		}
		fmt.Println(")")
		
		for i, movie := range movies {
			isLast := i == len(movies)-1
			prefix := "\u251c"
			if isLast {
				prefix = "\u2570"
			}
			
			fmt.Printf("%s\u2500\u2500 %s (%d)\n", prefix, movie.Title, movie.Year)
			if cfg.Safety.ShowDetails {
				indent := "\u2502   "
				if isLast {
					indent = "    "
				}
				
				if len(movie.TagNames) > 0 {
					fmt.Printf("%sTags: %s\n", indent, strings.Join(movie.TagNames, ", "))
				}
				
				dateInfo := fmt.Sprintf("Added: %s", movie.Added.Format("2006-01-02"))
				if !movie.FileImported.IsZero() {
					dateInfo += fmt.Sprintf(" | Imported: %s", movie.FileImported.Format("2006-01-02"))
				}
				fmt.Printf("%s%s\n", indent, dateInfo)
				
				if movie.WatchCount > 0 {
					watchInfo := fmt.Sprintf("Watched %dx", movie.WatchCount)
					if !movie.LastWatched.IsZero() {
						watchInfo += fmt.Sprintf(" (last: %s)", movie.LastWatched.Format("2006-01-02"))
					}
					fmt.Printf("%s%s\n", indent, watchInfo)
				}
			}
			if i < len(movies)-1 && cfg.Safety.ShowDetails {
				fmt.Printf("\u2502\n")
			}
		}
		fmt.Println()
	}

	return nil
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete movies matching the filter criteria",
	Long:  `Delete movies from your Radarr library that match the specified filter criteria.`,
	PreRunE: initializeApp,
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
	fmt.Printf("\nFound %d movie", len(moviesToDelete))
	if len(moviesToDelete) != 1 {
		fmt.Printf("s")
	}
	fmt.Println(" to delete")
	fmt.Println()
	
	for filterName, movies := range moviesByFilter {
		if len(movies) == 0 {
			continue
		}
		
		fmt.Printf("\u256d\u2500 Filter: %s (%d match", filterName, len(movies))
		if len(movies) != 1 {
			fmt.Printf("es")
		}
		fmt.Println(")")
		
		for i, movie := range movies {
			isLast := i == len(movies)-1
			prefix := "\u251c"
			if isLast {
				prefix = "\u2570"
			}
			fmt.Printf("%s\u2500\u2500 %s (%d)\n", prefix, movie.Title, movie.Year)
		}
		fmt.Println()
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
	PreRunE: initializeApp,
	RunE:  runTest,
}

func runTest(cmd *cobra.Command, args []string) error {
	// Test Radarr
	logger.Info().Str("url", cfg.Radarr.URL).Msg("Testing Radarr connection")
	logger.Info().Msg("✓ Radarr connection successful")

	// Test Tautulli if configured
	if tautulliClient != nil {
		logger.Info().Str("url", cfg.Tautulli.URL).Msg("Testing Tautulli connection")
		logger.Info().Msg("✓ Tautulli connection successful")
	} else {
		logger.Info().Msg("Tautulli integration: Not configured")
	}
	
	// Test Overseerr if configured
	if overseerrClient != nil {
		logger.Info().Str("url", cfg.Overseerr.URL).Msg("Testing Overseerr connection")
		logger.Info().Msg("✓ Overseerr connection successful")
	} else {
		logger.Info().Msg("Overseerr integration: Not configured")
	}

	// Test qBittorrent if configured
	if cfg.QBittorrent.URL != "" && cfg.QBittorrent.Username != "" {
		logger.Info().Str("url", cfg.QBittorrent.URL).Msg("Testing qBittorrent connection")
		// Try to create a client to test the connection
		_, err := qbittorrent.NewClient(cfg.QBittorrent.URL, cfg.QBittorrent.Username, cfg.QBittorrent.Password, logger)
		if err != nil {
			logger.Error().Err(err).Msg("✗ qBittorrent connection failed")
		} else {
			logger.Info().Msg("✓ qBittorrent connection successful")
		}
	} else {
		logger.Info().Msg("qBittorrent integration: Not configured")
	}

	return nil
}


// Import command variables
var (
	importPath   string
	importMovieID int64
	importMode   string
	autoApprove  bool
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Manually import movie files into Radarr",
	Long: `Scan a folder for movie files and import them into Radarr.
This command allows you to manually import files that were not automatically
processed by Radarr, such as files from qBittorrent that need to be re-imported.

Import modes:
- move: Moves files from source to Radarr's media folder (files are removed from source)
- copy: Creates hardlinks when possible (same filesystem), copies when not (preserves source files)

Use 'copy' mode when importing from qBittorrent to maintain seeding while saving disk space.`,
	PreRunE: initializeApp,
	RunE: runImport,
}

func init() {
	importCmd.Flags().StringVarP(&importPath, "path", "p", "", "path to scan for importable movies (required)")
	importCmd.Flags().Int64Var(&importMovieID, "movie-id", 0, "import files for a specific movie ID only")
	importCmd.Flags().StringVar(&importMode, "mode", "move", "import mode: 'move' (removes source) or 'copy' (hardlinks/copies)")
	importCmd.Flags().BoolVar(&autoApprove, "auto", false, "automatically import all valid files without confirmation")
	
	importCmd.MarkFlagRequired("path")
}

func runImport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Validate import mode
	if importMode != "move" && importMode != "copy" {
		return fmt.Errorf("invalid import mode: %s (must be 'move' or 'copy')", importMode)
	}
	
	// Create import options
	opts := radarr.ImportOptions{
		Path:       importPath,
		MovieID:    importMovieID,
		ImportMode: importMode,
	}
	
	// Scan for importable files
	logger.Info().Str("path", importPath).Msg("Scanning for importable movies")
	items, err := operations.ScanForImports(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to scan for imports: %w", err)
	}
	
	if len(items) == 0 {
		fmt.Println("No importable files found.")
		return nil
	}
	
	// Display found items
	operations.PrintImportableItems(items)
	
	// Check for items with rejections
	var validItems []*starr_radarr.ManualImportOutput
	var rejectedCount int
	for _, item := range items {
		if len(item.Rejections) > 0 {
			rejectedCount++
		} else {
			validItems = append(validItems, item)
		}
	}
	
	if rejectedCount > 0 {
		fmt.Printf("\n⚠️  %d file(s) cannot be imported due to rejections\n", rejectedCount)
	}
	
	if len(validItems) == 0 {
		fmt.Println("\nNo valid files to import.")
		return nil
	}
	
	// Confirm import
	if !autoApprove && !dryRun {
		fmt.Printf("\nImport %d file(s) using %s mode? [y/N]: ", len(validItems), importMode)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			logger.Info().Msg("Import cancelled by user")
			return nil
		}
	}
	
	if dryRun {
		fmt.Printf("\n[DRY RUN] Would import %d file(s) using %s mode\n", len(validItems), importMode)
		return nil
	}
	
	// Convert to import input format
	importInputs := operations.ConvertToImportInput(validItems, importMode)
	
	// Process imports
	if err := operations.ImportMovies(ctx, importInputs, opts); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}
	
	fmt.Printf("\n✓ Successfully imported %d file(s)\n", len(importInputs))
	return nil
}

