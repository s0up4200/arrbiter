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
./radarr-cleanup list
./radarr-cleanup delete --dry-run
./radarr-cleanup test
```

## Architecture Overview

This is a CLI tool for managing Radarr movies with advanced filtering and Tautulli integration for watch status checking. The codebase follows a modular design with clear separation of concerns.

### Core Components

1. **Filter System** (`filter/`)
   - Uses the expr expression language for powerful and flexible filtering
   - Custom helper functions: 
     - Tag: `hasTag()`
     - Watch: `watchedBy()`, `watchProgressBy()`, `watchCountBy()`
     - Date: `daysAgo()`, `monthsAgo()`, `yearsAgo()`, `parseDate()`
     - Rating: `imdbRating()`, `tmdbRating()`, `rottenTomatoesRating()`, `metacriticRating()`
     - Request: `requestedBy()`, `requestedAfter()`, `requestedBefore()`, `requestStatus()`, `approvedBy()`, `isRequested()`, `notRequested()`
   - Filter evaluation happens against `MovieInfo` structs with full access to all properties including ratings and request data
   - Supports complex expressions with arithmetic, comparisons, string operations, regex matching
   - Backwards compatibility layer (`compat.go`) converts legacy syntax to expr format

2. **API Clients**
   - **Radarr Client** (`radarr/`): Wraps golift/starr library, provides movie management
   - **Tautulli Client** (`tautulli/`): Custom implementation for Plex watch history
   - **Overseerr Client** (`overseerr/`): Fetches movie request data (who requested, when, status)
   - All clients support batch operations for efficiency

3. **Configuration** (`config/`)
   - Uses Viper for YAML configuration with defaults
   - Supports multiple config locations (current dir, ~/.radarr-cleanup/, /etc/radarr-cleanup/)
   - Config validation ensures API keys are set
   - Filters are defined as a simple map[string]string under the `filter:` section

4. **CLI Framework** (`cmd/`)
   - Uses Cobra for command structure
   - Three main commands: `list`, `delete`, `test`
   - No filter/preset flags - all filters from config are evaluated automatically
   - Results are grouped by which filter matched them

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
6. Each filter from config is evaluated against all movies using expr
7. Results are grouped by filter name for display (showing which filter matched)
8. For deletion, unique movies are collected (deduped) and deleted with optional file removal

## Common Development Tasks

### Adding New Filter Capabilities
1. Add new fields to `MovieInfo` struct in `radarr/client.go` if needed
2. Update `GetMovieInfo()` method to populate the new fields from Radarr API
3. Add helper functions to `filter/expr_filter.go` evaluation environment
4. Expose new properties in the expr evaluation environment (both at compile time and runtime)
5. Update README.md with new properties and example usage
6. Test with both simple and complex expressions

### Testing Filters
```bash
# Test filter syntax without making changes
./radarr-cleanup list

# Test with specific debug output
./radarr-cleanup list 2>&1 | grep "Processing filter"
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
- Always handle API errors gracefully with logging

## Configuration Notes

The tool expects sensitive API keys in `config.yaml` (gitignored). Example configuration is in `config.yaml.example`. 

Optional integrations:
- Tautulli integration can be disabled by setting `tautulli.enabled: false`
- Overseerr integration can be disabled by setting `overseerr.enabled: false`

Both integrations operate independently and can be enabled/disabled as needed.