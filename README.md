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

### Quick Start

Define your filters in `config.yaml`:

```yaml
filter:
  mathis: hasTag("trash") and watchedBy("myttolini")
  john: hasTag("action") and WatchCount > 5
  old_unwatched: Added < yearsAgo(1) and not Watched
```

Then run the tool:

```bash
# List all movies matching ANY of your filters
radarr-cleanup list

# Delete them (dry-run first to see what would be deleted)
radarr-cleanup delete

# Actually delete them
radarr-cleanup delete --no-dry-run
```

The tool will process ALL filters defined in your config and show results grouped by which filter matched.

### Other Examples

Test connection:
```bash
radarr-cleanup test
```

Skip confirmation when deleting:
```bash
radarr-cleanup delete --no-confirm
```

Keep files on disk when deleting from Radarr:
```bash
radarr-cleanup delete --delete-files=false
```

## Filter Expression Syntax

The tool uses the [expr](https://github.com/expr-lang/expr) expression language for filtering, which provides a powerful and flexible syntax.

### Available Movie Properties

```yaml
# Basic Properties
Title          # string - Movie title
Year           # int - Release year
Path           # string - File path on disk
IMDBID         # string - IMDb identifier (e.g., "tt1234567")
TMDBID         # int64 - The Movie Database ID

# Status Properties
HasFile        # bool - Whether movie has a file on disk
Watched        # bool - Whether movie has been watched by any user
WatchCount     # int - Total number of times watched by all users
WatchProgress  # float64 - Maximum watch progress percentage across all users

# Date Properties
Added          # time.Time - When movie was added to Radarr
FileImported   # time.Time - When the file was imported
LastWatched    # time.Time - When movie was last watched by any user

# Tag Properties
Tags           # []string - List of tag names (not IDs)
TagNames       # []string - Same as Tags (for compatibility)

# User-Specific Data
UserWatchData  # map[string]*UserWatchInfo - Per-user watch information

# Rating Properties
Ratings        # map[string]float64 - Map of rating source to value
Popularity     # float64 - Movie popularity score
```

### Helper Functions

```yaml
# Tag Functions
hasTag("tagname")              # Check if movie has a specific tag

# User Watch Functions
watchedBy("username")          # Check if specific user has watched (>85% by default)
watchCountBy("username")       # Get watch count for specific user
watchProgressBy("username")    # Get max progress percentage for user

# Date Functions
daysSince(date)                # Days since a given date
daysAgo(n)                     # Date n days ago
monthsAgo(n)                   # Date n months ago  
yearsAgo(n)                    # Date n years ago
parseDate("2023-01-01")        # Parse date string to time.Time
now()                          # Current time

# String Functions
contains(str, substr)          # Case-insensitive substring search
startsWith(str, prefix)        # Check string prefix (case-insensitive)
endsWith(str, suffix)          # Check string suffix (case-insensitive)
lower(str)                     # Convert to lowercase
upper(str)                     # Convert to uppercase

# Rating Functions
imdbRating()                   # Get IMDB rating (0 if not available)
tmdbRating()                   # Get TMDB rating (0 if not available)
rottenTomatoesRating()         # Get Rotten Tomatoes percentage (0 if not available)
metacriticRating()             # Get Metacritic score (0 if not available)
hasRating("source")            # Check if rating from source exists
getRating("source")            # Get rating value from any source (0 if not available)
```

### Examples

#### Basic Property Filtering

```yaml
# Filter by title
title_match: contains(Title, "Star Wars")
sequel_movies: contains(Title, "2") or contains(Title, "II")
the_movies: startsWith(Title, "The")

# Filter by year
old_movies: Year < 2000
recent_movies: Year >= 2020
specific_decade: Year >= 1980 and Year < 1990

# Filter by file path
mkv_files: endsWith(Path, ".mkv")
specific_folder: contains(Path, "/movies/action/")

# Filter by IDs
specific_imdb: IMDBID == "tt0111161"  # The Shawshank Redemption
has_tmdb: TMDBID > 0
```

#### Status and Watch Filtering

```yaml
# File status
missing_files: not HasFile
has_files: HasFile

# Watch status (any user)
watched_movies: Watched
unwatched_movies: not Watched
popular_movies: WatchCount > 3
barely_watched: WatchProgress < 20

# Specific user filtering
user_watched: watchedBy("john")
user_not_watched: not watchedBy("john")
user_watch_count: watchCountBy("john") >= 2
user_partial: watchProgressBy("john") > 0 and watchProgressBy("john") < 85

# Multiple users
either_user: watchedBy("john") or watchedBy("jane")
both_users: watchedBy("john") and watchedBy("jane")
john_not_jane: watchedBy("john") and not watchedBy("jane")
```

#### Date Filtering

```yaml
# Using specific dates
old_additions: Added < parseDate("2023-01-01")
recent_additions: Added > parseDate("2024-06-01")
date_range: Added >= parseDate("2024-01-01") and Added <= parseDate("2024-12-31")

# Using relative dates
last_30_days: Added > daysAgo(30)
last_week: daysSince(Added) <= 7
six_months_old: Added < monthsAgo(6)
over_year_old: Added < yearsAgo(1)

# File import dates
recent_imports: FileImported > daysAgo(7)
old_imports: daysSince(FileImported) > 365
never_imported: FileImported.IsZero()  # No file imported

# Last watched dates
recently_watched: daysSince(LastWatched) < 30
not_watched_year: daysSince(LastWatched) > 365
```

#### Tag Filtering

```yaml
# Single tag
action_movies: hasTag("action")
no_action: not hasTag("action")

# Multiple tags (OR)
action_or_thriller: hasTag("action") or hasTag("thriller")

# Multiple tags (AND)
action_and_good: hasTag("action") and hasTag("recommended")

# Tag combinations
user_tags: hasTag("10 - john") and not hasTag("keep")
cleanup_tags: hasTag("cleanup") or hasTag("remove") or hasTag("delete")

# Using array syntax (alternative)
has_any_tag: len(Tags) > 0
many_tags: len(Tags) >= 3
```

#### String Operations

```yaml
# Case-insensitive searches
marvel_movies: contains(Title, "marvel") or contains(Title, "avengers")
batman_movies: contains(lower(Title), "batman")

# Path operations
linux_isos: contains(Path, "linux") and endsWith(Path, ".iso")
downloads_folder: startsWith(Path, "/downloads/")

# Title patterns
numbered_sequels: Title matches ".*\\s\\d+$"  # Ends with number
year_in_title: Title matches ".*\\(\\d{4}\\)"  # Contains year in parentheses
```

#### Complex Real-World Examples

```yaml
# Old unwatched movies with specific tags
cleanup_candidates: |
  hasTag("cleanup") and 
  not Watched and 
  Added < monthsAgo(6)

# Movies partially watched by guests
guest_abandoned: |
  watchProgressBy("guest") > 10 and 
  watchProgressBy("guest") < 70 and
  daysSince(LastWatched) > 30

# High quality files nobody watches
wasted_space: |
  contains(Path, "2160p") and 
  WatchCount == 0 and 
  Added < monthsAgo(3)

# Kids movies adults watched
kids_watched_by_adults: |
  hasTag("kids") and 
  (watchedBy("mom") or watchedBy("dad")) and
  not watchedBy("child1")

# Incomplete series
incomplete_series: |
  (contains(Title, "Part 1") or contains(Title, "Chapter 1")) and
  not Watched

# Foreign films with low engagement  
foreign_unwatched: |
  not contains(Path, "/english/") and
  WatchCount < 2 and
  daysSince(Added) > 60

# Recently added but quickly abandoned
quick_abandons: |
  daysSince(Added) < 30 and
  WatchProgress > 0 and
  WatchProgress < 30 and
  daysSince(LastWatched) > 7

# Using Movie prefix for direct access
movie_prefix: Movie.Title == "The Matrix" and Movie.Year == 1999
```

#### Rating-Based Filtering

```yaml
# High quality movies only
high_rated: imdbRating() >= 7.5 and rottenTomatoesRating() >= 80

# Low rated movies to clean up
low_rated: imdbRating() < 5.0 or metacriticRating() < 40

# Movies with no ratings
unrated: not hasRating("imdb") and not hasRating("tmdb")

# Popular movies
popular: Popularity > 100 and tmdbRating() > 7

# Critics vs audience disagreement
controversial: abs(rottenTomatoesRating() - imdbRating() * 10) > 30

# Highly rated but unwatched
highly_rated_unwatched: |
  imdbRating() >= 8.0 and
  not Watched and
  Added < monthsAgo(3)

# Low IMDB but high RT score (critical darlings)
critical_darlings: |
  imdbRating() < 6.5 and
  rottenTomatoesRating() > 85

# Check specific rating exists
has_metacritic: hasRating("metacritic")

# Use any rating source dynamically
any_high_rating: |
  getRating("imdb") > 8 or
  getRating("tmdb") > 8 or
  getRating("metacritic") > 80

# Combine ratings with other properties
good_action_movies: |
  hasTag("action") and
  imdbRating() >= 7.0 and
  rottenTomatoesRating() >= 70
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
No additional options - processes all filters from config

### Delete Command
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