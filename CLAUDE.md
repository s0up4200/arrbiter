# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the binary
go build

# Download dependencies
go mod download
go mod tidy

# Run tests
go test ./...

# Run a specific test package
go test ./filter/...
go test ./radarr/...

# Run with verbose output
go test -v ./...

# Install globally
go install

# Run the binary directly after building
./arrbiter list
./arrbiter delete --dry-run
./arrbiter test
./arrbiter hardlink
./arrbiter import --path /downloads
```

## Architecture Overview

This is a CLI tool for managing Radarr movies with advanced filtering, Tautulli integration for watch status checking, Overseerr integration for request tracking, and qBittorrent integration for hardlink management. The codebase follows a modular design with clear separation of concerns.

### Core Components

1. **Filter System** (`filter/`)
   - Uses the expr expression language for powerful and flexible filtering
   - Custom helper functions: 
     - Tag: `hasTag()`
     - Watch: `watchedBy()`, `watchProgressBy()`, `watchCountBy()`
     - Date: `daysAgo()`, `monthsAgo()`, `yearsAgo()`, `parseDate()`
     - Rating: `imdbRating()`, `tmdbRating()`, `rottenTomatoesRating()`, `metacriticRating()`
     - Request: `requestedBy()`, `requestedAfter()`, `requestedBefore()`, `requestStatus()`, `approvedBy()`, `isRequested()`, `notRequested()`, `notWatchedByRequester()`, `watchedByRequester()`
   - Filter evaluation happens against `MovieInfo` structs with full access to all properties including ratings and request data
   - Supports complex expressions with arithmetic, comparisons, string operations, regex matching
   - Backwards compatibility layer (`compat.go`) converts legacy syntax to expr format

2. **API Clients**
   - **Radarr Client** (`radarr/`): Wraps golift/starr library, provides movie management
   - **Tautulli Client** (`tautulli/`): Custom implementation for Plex watch history
   - **Overseerr Client** (`overseerr/`): Fetches movie request data (who requested, when, status)
   - **qBittorrent Client** (`qbittorrent/`): Uses autobrr/go-qbittorrent for torrent management
   - All clients support batch operations for efficiency

3. **Configuration** (`config/`)
   - Uses Viper for YAML configuration with defaults
   - Supports multiple config locations (current dir, ~/.radarr-cleanup/, /etc/radarr-cleanup/)
   - Config validation ensures API keys are set
   - Filters are defined as a simple map[string]string under the `filter:` section

4. **CLI Framework** (`cmd/`)
   - Uses Cobra for command structure
   - Main commands: `list`, `delete`, `test`, `import`, `hardlink`
   - No filter/preset flags - all filters from config are evaluated automatically
   - Results are grouped by which filter matched them

5. **Hardlink Detection** (`hardlink/`)
   - Unix-specific implementation using syscall for hardlink detection
   - Functions: `HasHardlinks()`, `GetHardlinkCount()`, `AreHardlinked()`
   - Windows returns unsupported error (hardlinks work differently on Windows)

### Key Design Decisions

1. **Per-User Watch Data**: `MovieInfo` contains a `UserWatchData` map to store individual user watch status, enabling filters like `watchedBy("username")`

2. **Safety First**: Dry-run mode is enabled by default, requires explicit `--no-dry-run` flag to delete

3. **Batch Processing**: Tautulli client fetches all history at once and processes in-memory for better performance

4. **Automatic Filter Processing**: All filters defined in config are evaluated - no need for CLI flags to select filters

5. **Backwards Compatibility**: Legacy filter syntax (e.g., `tag:"value"`) is automatically converted to expr syntax

### Filter Expression System

The filter system uses the expr expression language (github.com/expr-lang/expr):
- Provides powerful, type-safe expression evaluation
- Custom helper functions for common operations (hasTag, watchedBy, date helpers, rating functions)
- Direct access to all MovieInfo properties in expressions
- Supports arithmetic, comparisons, string operations, regex matching
- Array operations with filter(), any(), all() functions and regex patterns

### Integration Flow

1. Config loads and validates API connections (requires API keys)
2. List/Delete commands fetch all movies from Radarr in a single API call
3. Tags are fetched separately and mapped to movie TagNames for filtering
4. If Tautulli enabled, enriches movies with per-user watch status via batch history API
5. If Overseerr enabled, enriches movies with request data (requester, date, status) via API
6. If qBittorrent enabled (for hardlink command), checks torrent status for non-hardlinked files
7. Each filter from config is evaluated against all movies using expr
8. Results are grouped by filter name for display (showing which filter matched)
9. For deletion, unique movies are collected (deduped) and deleted with optional file removal

### Hardlink Management Flow

1. Scan all movies from Radarr and check hardlink count using system calls
2. For non-hardlinked movies (Nlink = 1), check if they exist in qBittorrent
3. If found in qBittorrent and seeding:
   - Use Radarr's manual import to re-import the file
   - This creates a hardlink between qBittorrent and Radarr directories
4. If not found in qBittorrent:
   - Optionally delete the file and trigger a new search in Radarr
5. Process movies interactively with user confirmation for each action

## Common Development Tasks

### Adding New Filter Capabilities
1. Add new fields to `MovieInfo` struct in `radarr/client.go` if needed
2. Update `GetMovieInfo()` method to populate the new fields from Radarr API
3. Add helper functions to `filter/expr_filter.go` evaluation environment
4. Expose new properties in the expr evaluation environment (both at compile time and runtime)
5. Update README.md with new properties and example usage
6. Test with both simple and complex expressions

### Adding New Commands
1. Create new command file in `cmd/` directory (e.g., `hardlink.go`)
2. Define command structure using Cobra with appropriate flags
3. Implement business logic in relevant package (e.g., `radarr/hardlink_operations.go`)
4. Add any new API clients in their own packages (e.g., `qbittorrent/`)
5. Update configuration types in `config/types.go` if needed
6. Initialize new clients in `cmd/root.go` during app initialization
7. Update documentation in README.md and CLAUDE.md

### Testing Filters
```bash
# Test filter syntax without making changes
./arrbiter list

# Test with specific debug output
./arrbiter list 2>&1 | grep "Processing filter"
```

### Filter Expression Details

The expr filter system provides access to:
- All MovieInfo struct fields directly (e.g., `Title`, `Year`)
- Helper functions defined in the evaluation environment
- The full Movie object via `Movie.FieldName` syntax
- Standard expr operators and functions

Key implementation notes:
- Properties are added to the evaluation environment in `expr_filter.go` 
- Helper functions are defined both at compile time and evaluation time
- The `IsZero()` method works on time fields to check if they're unset
- String comparisons in helper functions are case-insensitive
- Date arithmetic uses Go's time package functions
- Ratings are exposed from Radarr API (IMDB, TMDB, Rotten Tomatoes, Metacritic)
- Regex patterns in array operations require proper escaping in YAML (use single quotes)

When modifying API interactions:
- Radarr operations should go through the Operations struct for consistency
- Tautulli client should maintain batch processing for performance
- qBittorrent client uses autobrr/go-qbittorrent library
- Always handle API errors gracefully with logging
- New clients should follow the existing pattern of wrapping external libraries

## Configuration Notes

The tool expects sensitive API keys in `config.yaml` (gitignored). Example configuration is in `config.yaml.example`. 

Optional integrations:
- Tautulli integration is enabled automatically when both `url` and `api_key` are provided
- Overseerr integration is enabled automatically when both `url` and `api_key` are provided
- qBittorrent integration is enabled automatically when `url` and `username` are provided

All integrations operate independently - just leave the configuration empty if you don't want to use them.

## Logging System

The codebase uses zerolog for structured logging with the following characteristics:
- Console output only (no JSON mode) with tree-style formatting
- Smart terminal detection for color support using go-isatty
- Log levels: trace, debug, info (default), warn, error
- Timestamps in HH:MM:SS format
- Clear separation between user output (fmt) and diagnostic logs (zerolog)

## Release Process

The project uses GitHub Actions with GoReleaser for automated releases:
- Triggered on tags matching `v*` pattern
- Go version: 1.24
- Builds for multiple platforms via `.goreleaser.yml`
- Creates GitHub releases with binaries
- Homebrew tap formula in `Casks/arrbiter.rb`