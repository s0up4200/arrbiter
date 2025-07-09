# Arrbiter

> Your media library's arbiter of taste

A CLI tool to manage and clean up Radarr movies based on advanced filter criteria, with optional Tautulli and Overseerr integration for watch status and request tracking.

## Features

- **Advanced Filtering**: Filter movies by tags, date added, date imported, and watch status
- **Tautulli Integration**: Check if movies have been watched before deleting
- **Overseerr Integration**: Filter based on who requested movies and when
- **Logical Operators**: Combine filters using AND, OR, and NOT operators
- **Preset Filters**: Define reusable filter expressions in configuration
- **Safety Features**: Dry-run mode, confirmation prompts, watched movie warnings
- **Flexible Configuration**: YAML-based configuration with sensible defaults

## Installation

### Homebrew

```bash
brew tap s0up4200/arrbiter https://github.com/s0up4200/arrbiter
brew install --cask arrbiter
```

### Manual Download

Download the latest release for your platform:

#### Linux (x86_64)
```bash
wget $(curl -s https://api.github.com/repos/s0up4200/arrbiter/releases/latest | grep download | grep Linux_x86_64 | cut -d\" -f4)
sudo tar -C /usr/local/bin -xzf arrbiter*.tar.gz
```

#### FreeBSD (x86_64)
```bash
wget $(curl -s https://api.github.com/repos/s0up4200/arrbiter/releases/latest | grep download | grep Freebsd_x86_64 | cut -d\" -f4)
sudo tar -C /usr/local/bin -xzf arrbiter*.tar.gz
```

#### Windows (x86_64)
```powershell
# PowerShell
$latest = Invoke-RestMethod -Uri https://api.github.com/repos/s0up4200/arrbiter/releases/latest
$url = $latest.assets | Where-Object { $_.name -match "Windows_x86_64.zip" } | Select-Object -ExpandProperty browser_download_url
Invoke-WebRequest -Uri $url -OutFile "arrbiter.zip"
# Extract and add to PATH manually
```

> **Note**: More platforms and architectures (macOS, ARM, etc.) are available on the [releases page](https://github.com/s0up4200/arrbiter/releases).

### Go Install

```bash
go install github.com/s0up4200/arrbiter@latest
```

### Build from source

```bash
git clone https://github.com/s0up4200/arrbiter.git
cd arrbiter
go build
```

## Configuration

1. Copy the example configuration:
   ```bash
   cp config.yaml.example config.yaml
   ```

2. Edit `config.yaml` with your Radarr and optionally Tautulli/Overseerr details:
   ```yaml
   radarr:
     url: http://localhost:7878
     api_key: your-api-key-here
   
   # Optional: Add Tautulli for watch tracking
   tautulli:
     url: http://localhost:8181
     api_key: your-tautulli-api-key
     min_watch_percent: 85  # Consider watched if >85% viewed
     
   # Optional: Add Overseerr for request tracking
   overseerr:
     url: http://localhost:5055
     api_key: your-overseerr-api-key
   ```

The tool will look for `config.yaml` in:
- Current directory
- `~/.config/arrbiter/`
- `~/.arrbiter/`

If no config file is found, a default one will be created at `~/.config/arrbiter/config.yaml`.

## Usage

### Quick Start

Define your filters in `config.yaml`:

```yaml
filter:
  # Remove movies requested but not watched within 30 days
  unwatched_requests: notWatchedByRequester() and Added < daysAgo(30)
  
  # Clean up low-rated movies nobody requested
  poor_quality: imdbRating() < 5.5 and notRequested() and Added < daysAgo(30)
  
  # Free up space from old unwatched content
  space_cleanup: not Watched and Added < monthsAgo(3) and not hasTag("keep")
```

Then run the tool:

```bash
# List all movies matching ANY of your filters
arrbiter list

# Delete them (dry-run first to see what would be deleted)
arrbiter delete

# Actually delete them
arrbiter delete --no-dry-run
```

The tool will process ALL filters defined in your config and show results grouped by which filter matched.

### Other Examples

Test connection:
```bash
arrbiter test
```

Skip confirmation when deleting:
```bash
arrbiter delete --no-confirm
```

Keep files on disk when deleting from Radarr:
```bash
arrbiter delete --delete-files=false
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

# Request Properties (Overseerr Integration)
RequestedBy      # string - Username of who requested the movie
RequestedByEmail # string - Email of requester
RequestDate      # time.Time - When the movie was requested
RequestStatus    # string - Status from Overseerr (PENDING, APPROVED, AVAILABLE)
ApprovedBy       # string - Who approved the request
IsAutoRequest    # bool - Whether it was an automatic request
IsRequested      # bool - Whether movie was requested via Overseerr
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

# Request Functions (Overseerr Integration)
requestedBy("username")        # Check if movie was requested by specific user
requestedAfter(date)           # Movies requested after date
requestedBefore(date)          # Movies requested before date
requestStatus("AVAILABLE")     # Filter by request status
approvedBy("admin")            # Filter by who approved
isRequested()                  # Check if movie was requested vs manually added
notRequested()                 # Movies added directly to Radarr
notWatchedByRequester()        # Movies where the requester hasn't watched them
watchedByRequester()           # Movies where the requester has watched them
```

### Practical Filter Examples

#### Request Accountability
*Ensure users watch what they request*

```yaml
# Movies requested but not watched by requester after reasonable time
unwatched_requests_30d: |
  notWatchedByRequester() and 
  Added < daysAgo(30)

# Movies requested by specific user who hasn't watched them
user_unwatched_requests: |
  requestedBy("john") and 
  not watchedBy("john") and 
  Added < daysAgo(14)

# Successfully watched requests (can be excluded from cleanup)
successful_requests: |
  watchedByRequester() and 
  watchProgressBy(RequestedBy) > 90
```

#### Storage Management
*Free up disk space intelligently*

```yaml
# Old movies nobody has watched
space_wasters: |
  not Watched and 
  Added < monthsAgo(3) and 
  not hasTag("keep")

# Large files with low engagement
unwatched_4k: |
  contains(Path, "2160p") and 
  WatchCount < 2 and 
  Added < monthsAgo(2)

# Movies watched once and forgotten
one_time_watches: |
  WatchCount == 1 and 
  daysSince(LastWatched) > 180
```

#### Quality Control
*Remove low-quality content unless specifically wanted*

```yaml
# Low-rated movies nobody requested
poor_quality_unrequested: |
  imdbRating() < 5.5 and 
  notRequested() and 
  Added < daysAgo(30)

# Low-rated movies even if requested (but give them time)
poor_quality_any: |
  imdbRating() < 5 and 
  not Watched and 
  Added < monthsAgo(2)

# Movies with poor ratings across multiple sources
universally_bad: |
  imdbRating() < 5 and 
  rottenTomatoesRating() < 40 and 
  not hasTag("guilty-pleasure")
```

#### User Management
*Handle inactive users and their content*

```yaml
# Requests from users who left the server
inactive_user_cleanup: |
  requestedBy("former_user") and 
  not Watched and 
  Added < monthsAgo(1)

# Guest requests that weren't watched
guest_cleanup: |
  requestedBy("guest") and 
  not watchedBy("guest") and 
  Added < daysAgo(7)
```

#### Time-Based Cleanup Policies
*Progressive cleanup based on age and engagement*

```yaml
# 30-day policy: Remove if unwatched after a month
cleanup_30d: |
  not Watched and 
  Added < daysAgo(30) and 
  imdbRating() < 7

# 90-day policy: Remove poorly rated movies
cleanup_90d: |
  not Watched and 
  Added < daysAgo(90) and 
  imdbRating() < 6

# 180-day policy: Remove anything still unwatched
cleanup_180d: |
  not Watched and 
  Added < daysAgo(180)
```

#### Tag-Based Management
*Use tags for organization and protection*

```yaml
# Movies tagged for removal
tagged_removal: |
  hasTag("remove") or 
  hasTag("cleanup")

# Protect tagged movies from all other filters
protected: |
  not hasTag("keep") and 
  not hasTag("favorite") and 
  # ... your other conditions
```

#### Combined Real-World Scenarios

```yaml
# Smart cleanup: Unwatched + Low rated + Not recent + Not requested
smart_cleanup: |
  not Watched and
  imdbRating() < 6.5 and
  Added < monthsAgo(2) and
  notRequested()

# Request accountability with ratings consideration
request_quality_check: |
  notWatchedByRequester() and
  imdbRating() < 6 and
  Added < daysAgo(30)

# Space saver: Old, unwatched, low-rated, not protected
aggressive_cleanup: |
  not Watched and
  (imdbRating() < 6 or not hasRating("imdb")) and
  Added < monthsAgo(3) and
  not hasTag("keep") and
  WatchCount == 0
```

#### Understanding Request Watch Functions

**`notWatchedByRequester()`**
- Returns `true` only if the movie was requested AND the requester hasn't watched it (< 85% progress)
- Returns `false` if the movie wasn't requested or if the requester has watched it
- Useful for cleaning up movies that users requested but never watched

**`watchedByRequester()`**
- Returns `true` only if the movie was requested AND the requester has watched it (≥ 85% progress)
- Returns `false` if the movie wasn't requested or if the requester hasn't watched it
- Useful for finding successful requests where the requester actually watched the movie

**Important Notes:**
- Both functions require the movie to be requested through Overseerr
- They check the watch status of specifically the person who requested it
- Using `!notWatchedByRequester()` matches movies that either weren't requested OR were requested and watched
- Using `!watchedByRequester()` matches movies that either weren't requested OR were requested but not watched


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
  url: http://localhost:8181
  api_key: your-tautulli-api-key
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

## Overseerr Integration

When Overseerr is enabled, the tool will retrieve request information for movies to help with filtering decisions. This allows you to filter based on who requested movies, when they were requested, and their request status.

### Configuration

```yaml
overseerr:
  url: http://localhost:5055
  api_key: your-overseerr-api-key
```

### How It Works

1. The tool queries Overseerr for all movie requests via the API
2. Movies are matched by TMDB ID
3. Request information includes who requested, when, status, and who approved
4. The most recent request is used if multiple exist for the same movie

### Request Properties Available

- `RequestedBy`: Username of the person who requested the movie
- `RequestDate`: When the movie was requested
- `RequestStatus`: Current status (PENDING, APPROVED, AVAILABLE, etc.)
- `ApprovedBy`: Username of who approved the request (if applicable)
- `IsRequested`: Whether the movie came through Overseerr

### Use Cases

This integration enables powerful filtering scenarios:
- Clean up movies requested by users who no longer use the service
- Remove old requests that were never watched
- Protect movies that were specifically requested by certain users
- Differentiate between requested content and manually added movies

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