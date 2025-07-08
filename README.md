# Radarr Cleanup

A CLI tool to manage and clean up Radarr movies based on advanced filter criteria, with optional Tautulli integration for watch status.

## Features

- **Advanced Filtering**: Filter movies by tags, date added, date imported, and watch status
- **Tautulli Integration**: Check if movies have been watched before deleting
- **Logical Operators**: Combine filters using AND, OR, and NOT operators
- **Preset Filters**: Define reusable filter expressions in configuration
- **Safety Features**: Dry-run mode, confirmation prompts, watched movie warnings
- **Flexible Configuration**: YAML-based configuration with sensible defaults

## Installation

```bash
go install github.com/soup/radarr-cleanup@latest
```

Or build from source:

```bash
git clone https://github.com/soup/radarr-cleanup.git
cd radarr-cleanup
go build
```

## Configuration

1. Copy the example configuration:
   ```bash
   cp config.yaml.example config.yaml
   ```

2. Edit `config.yaml` with your Radarr and optionally Tautulli details:
   ```yaml
   radarr:
     url: http://localhost:7878
     api_key: your-api-key-here
   
   tautulli:
     enabled: true
     url: http://localhost:8181
     api_key: your-tautulli-api-key
   ```

The tool will look for `config.yaml` in:
- Current directory
- `~/.radarr-cleanup/`
- `/etc/radarr-cleanup/`

## Usage

### Quick Start - Simple Use Case

If you want to delete movies with a specific Radarr tag that a user has already watched:

```bash
# List movies with tag "01 - snekkern" that user "snekkern" has watched
radarr-cleanup list --filter 'tag:"01 - snekkern" AND watched_by:"snekkern"'

# Delete them (dry-run first to see what would be deleted)
radarr-cleanup delete --filter 'tag:"01 - snekkern" AND watched_by:"snekkern"'

# Actually delete them
radarr-cleanup delete --filter 'tag:"01 - snekkern" AND watched_by:"snekkern"' --no-dry-run
```

### Other Examples

Test connection:
```bash
radarr-cleanup test
```

List unwatched movies with a tag:
```bash
radarr-cleanup list --filter 'tag:"01 - snekkern" AND NOT watched_by:"snekkern"'
```

Use a preset from config:
```bash
radarr-cleanup list --preset snekkern_watched
```

Delete old imports nobody has watched:
```bash
radarr-cleanup delete --filter 'imported_before:"2023-01-01" AND watched:false'
```

## Filter Expression Syntax

### Basic Filters

- **Tag filter**: `tag:"tagname"`
- **Date added**: `added_before:"2023-01-01"` or `added_after:"2023-01-01"`
- **Date imported**: `imported_before:"2023-01-01"` or `imported_after:"2023-01-01"`
- **Watch status**: `watched:true` or `watched:false` (any user)
- **Watch count**: `watch_count:>0` or `watch_count:>=2` (total across all users)
- **Watched by user**: `watched_by:"username"` (specific user has watched)
- **User watch count**: `watch_count_by:"username">3` (specific user watched N times)

### Logical Operators

- **AND**: Combine multiple conditions that must all be true
- **OR**: At least one condition must be true
- **NOT**: Negate a condition
- **Parentheses**: Group conditions for complex logic

### Examples

```bash
# Movies with specific tag
tag:"01 - snekkern"

# Movies added after a date
added_after:"2024-01-01"

# Complex filter with AND
tag:"01 - snekkern" AND added_after:"2024-01-01"

# Multiple conditions with OR
(tag:"cleanup" AND imported_before:"2023-12-31") OR (tag:"archive" AND added_before:"2023-06-01")

# Negation
NOT tag:"keep" AND added_before:"2023-01-01"

# Movies without specific tag
tag!:"keep"

# Unwatched movies with specific tag
tag:"cleanup" AND watched:false

# Movies watched more than once
watch_count:>1

# Movies watched by specific user
tag:"01 - snekkern" AND watched_by:"snekkern"

# Movies NOT watched by a specific user
NOT watched_by:"guest"

# Movies watched by user more than 3 times
watch_count_by:"poweruser">3
```

## Safety Features

1. **Dry Run Mode**: Enabled by default, shows what would be deleted without making changes
2. **Confirmation Prompts**: Asks for confirmation before deleting (can be disabled)
3. **Watched Movie Warnings**: Warns when attempting to delete watched movies
4. **Detailed Logging**: Structured logging with adjustable levels
5. **File Deletion Control**: Choose whether to delete files from disk

## Command Line Options

### Global Options
- `--config`: Specify config file location
- `--dry-run, -d`: Perform a dry run without making changes

### List Command
- `--filter, -f`: Filter expression
- `--preset, -p`: Use a preset filter from config

### Delete Command
- `--filter, -f`: Filter expression
- `--preset, -p`: Use a preset filter from config
- `--no-confirm`: Skip confirmation prompt
- `--delete-files`: Also delete movie files from disk (default: true)
- `--ignore-watched`: Delete movies even if they have been watched

## Tautulli Integration

When Tautulli is enabled, the tool will check if movies have been watched before deletion. This helps prevent accidentally deleting movies that users have already viewed.

### Configuration

```yaml
tautulli:
  enabled: true
  url: http://localhost:8181
  api_key: your-tautulli-api-key
  
  watch_check:
    enabled: true
    min_watch_percent: 85  # Consider watched if >85% viewed
```

### How It Works

1. The tool queries Tautulli for all movie watch history
2. Movies are matched by IMDB ID or title
3. A movie is considered "watched" if viewed past the `min_watch_percent` threshold
4. When deleting, watched movies trigger a warning unless `--ignore-watched` is used

### User-Specific Filtering

The tool now supports filtering by specific Plex users:
- Use `watched_by:"username"` to check if a specific user has watched a movie
- Use `watch_count_by:"username">N` to check how many times a user has watched
- The general `watched:true` filter checks if ANY user has watched the movie

## Development

### Dependencies

- `github.com/spf13/cobra`: CLI framework
- `github.com/spf13/viper`: Configuration management
- `github.com/rs/zerolog`: Structured logging
- `golift.io/starr`: Radarr API client

### Building

```bash
go mod download
go build
```

### Running Tests

```bash
go test ./...
```

## License

MIT