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
```

## Architecture Overview

This is a CLI tool for managing Radarr movies with advanced filtering and Tautulli integration for watch status checking. The codebase follows a modular design with clear separation of concerns.

### Core Components

1. **Filter System** (`filter/`)
   - Custom expression parser that supports complex boolean logic (AND, OR, NOT, parentheses)
   - Tokenizer converts filter strings into tokens, parser builds an AST-like structure
   - Special handling for `watch_count_by:"username">N` syntax that requires parsing both username and count
   - Filter evaluation happens against `MovieInfo` structs

2. **API Clients**
   - **Radarr Client** (`radarr/`): Wraps golift/starr library, provides movie management
   - **Tautulli Client** (`tautulli/`): Custom implementation for Plex watch history
   - Both clients support batch operations for efficiency

3. **Configuration** (`config/`)
   - Uses Viper for YAML configuration with defaults
   - Supports multiple config locations (current dir, ~/.radarr-cleanup/, /etc/radarr-cleanup/)
   - Config validation ensures API keys are set

4. **CLI Framework** (`cmd/`)
   - Uses Cobra for command structure
   - Three main commands: `list`, `delete`, `test`
   - Global flags override config values when specified

### Key Design Decisions

1. **Per-User Watch Data**: `MovieInfo` contains a `UserWatchData` map to store individual user watch status, enabling filters like `watched_by:"username"`

2. **Safety First**: Dry-run mode is enabled by default, requires explicit `--no-dry-run` flag to delete

3. **Batch Processing**: Tautulli client fetches all history at once and processes in-memory for better performance

4. **Parser Field Naming**: Parser struct uses `currentIdx` for the token index to avoid conflict with `current()` method that returns the current character

### Filter Expression Parsing

The filter parser uses a recursive descent approach:
- `parseExpression()` → `parseOr()` → `parseAnd()` → `parseNot()` → `parsePrimary()`
- Special case handling in `parsePrimary()` for `watch_count_by` fields that need additional tokens

### Integration Flow

1. Config loads and validates API connections
2. Operations fetches movies from Radarr
3. If Tautulli enabled, enriches movies with watch status using batch API call
4. Filter expressions are parsed and evaluated against enriched movie data
5. Matching movies are displayed or deleted based on command

## Common Development Tasks

When adding new filter fields:
1. Add the field type constant in `filter/types.go`
2. Update parser in `filter/parser.go` to handle the new field
3. Implement evaluation logic in `filter/filter.go`
4. Update `MovieInfo` struct if new data is needed

When modifying API interactions:
- Radarr operations should go through the Operations struct for consistency
- Tautulli client should maintain batch processing for performance
- Always handle API errors gracefully with logging

## Configuration Notes

The tool expects sensitive API keys in `config.yaml` (gitignored). Example configuration is in `config.yaml.example`. The Tautulli integration can be disabled entirely by setting `tautulli.enabled: false`.