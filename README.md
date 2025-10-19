# Arrbiter

[![GitHub release](https://img.shields.io/github/release/s0up4200/arrbiter.svg)](https://github.com/s0up4200/arrbiter/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/s0up4200/arrbiter)](https://goreportcard.com/report/github.com/s0up4200/arrbiter)
[![License](https://img.shields.io/github/license/s0up4200/arrbiter.svg)](LICENSE)

> Your media library's arbiter of taste

## The Problem

Your Radarr library keeps growing, but disk space doesn't. You've got:
- **Movies requested but never watched** taking up precious storage
- **Low-rated content** that nobody wants to see again  
- **No easy way to clean up** without manually checking each movie

## The Solution

Arrbiter is a smart CLI tool that automatically identifies and removes movies based on your criteria:
- **Smart filtering** using ratings, watch history, and request data
- **Integrates with your stack** (Radarr, Tautulli, Overseerr, qBittorrent)
- **Safety first** with dry-run mode and confirmation prompts
- **Powerful expressions** for complex cleanup rules

> **Important**: This tool can permanently delete movies from your Radarr library and disk. Always test in dry-run mode first (which is the default)!

## Quick Example

```bash
# Show what would be deleted (safe to run)
arrbiter list

# Show what would be deleted in dry-run mode (safe - default behavior)
arrbiter delete

# Force dry-run mode (always safe)
arrbiter delete --dry-run

# To actually delete, set dry_run: false in config.yaml, then:
arrbiter delete
```

## Key Features

- **Smart Cleanup**: Remove unwatched, low-rated, or old content automatically
- **Request Tracking**: Clean up movies people requested but never watched
- **Watch Analytics**: Integration with Tautulli for viewing history
- **Quality Upgrades**: Find and upgrade movies missing custom formats
- **Hardlink Management**: Fix storage issues with qBittorrent integration
- **Multiple Safety Nets**: Dry-run mode, confirmations, and detailed logging
- **Highly Configurable**: Powerful filter expressions for any cleanup scenario

## Prerequisites

### Required
- **Radarr** v3+ with API access
- **Operating System**: Linux, macOS, FreeBSD, or Windows

### Optional (for enhanced features)
- **Tautulli**: For watch history tracking and user-specific filtering
- **Overseerr/Jellyseerr**: For request tracking and accountability features  
- **qBittorrent**: For hardlink management and storage optimization
- **Same filesystem**: qBittorrent and Radarr must be on the same filesystem for hardlinks

### Permissions Needed
- Read access to Radarr API
- Write access to Radarr (for deletions)
- Network access to optional services (Tautulli, Overseerr, qBittorrent)

> **Tip**: You can start with just Radarr and add other integrations later!

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

## 5-Minute Quick Start

### Step 1: Create Configuration
Create a `config.yaml` file with your Radarr details:

```yaml
radarr:
  url: http://localhost:7878          # Your Radarr URL
  api_key: your-radarr-api-key-here  # Get this from Radarr Settings > General

# Define some basic cleanup filters
filter:
  # Remove old unwatched movies (safe starter filter)
  old_unwatched: not Watched and Added < monthsAgo(6)
  
  # Remove low-rated movies that are old
  low_rated_old: imdbRating() < 5.5 and Added < monthsAgo(3)
```

> **Config Location**: The tool looks for `config.yaml` in the current directory, `~/.config/arrbiter/`, or `~/.arrbiter/`

### Step 2: Test Your Setup
```bash
# Test connection to Radarr
arrbiter test

# See what movies match your filters (completely safe)
arrbiter list
```

### Step 3: Run Your First Cleanup (Safely!)
```bash
# Shows what WOULD be deleted (safe - default behavior)
arrbiter delete

# To actually delete movies:
# 1. First, edit your config.yaml and set:
#    safety:
#      dry_run: false
# 2. Then run the same command:
arrbiter delete
```

### Step 4: Configure for Actual Deletions (When Ready)
To switch from dry-run mode to actual deletions, edit your `config.yaml`:

```yaml
# Change this setting to allow actual deletions
safety:
  dry_run: false          # Set to false to enable actual deletions
  confirm_delete: true    # Keep confirmations for safety
```

### Step 5: Add More Integrations (Optional)
Add these to your `config.yaml` for enhanced features:

```yaml
# Add Tautulli for watch tracking
tautulli:
  url: http://localhost:8181
  api_key: your-tautulli-api-key

# Add Overseerr for request tracking  
overseerr:
  url: http://localhost:5055
  api_key: your-overseerr-api-key
```

> **You're ready!** Start with the basic filters above, then explore the [advanced examples](#advanced-filter-examples) below.

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

# Show what would be deleted (dry-run mode - default behavior)
arrbiter delete

# To actually delete them, set dry_run: false in config.yaml first, then:
arrbiter delete

# Manually import movie files from a folder
arrbiter import --path /path/to/movies

# Import files for a specific movie only
arrbiter import --path /downloads --movie-id 123

# Import using copy mode to create hardlinks (useful for qBittorrent seeding)
arrbiter import --path /downloads --mode copy --auto

# Manage non-hardlinked movies (requires qBittorrent)
arrbiter hardlink

# Skip confirmation prompts
arrbiter hardlink --no-confirm

# Dry run to see what would be done
arrbiter hardlink --dry-run

# Find and upgrade movies missing custom formats
arrbiter upgrade

# Upgrade in unattended mode (upgrade 5 movies)
arrbiter upgrade --unattended 5

# Override match mode and disable monitoring
arrbiter upgrade --match any --no-monitor
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

## Basic Filter Examples

*Start with these common cleanup scenarios:*

```yaml
filter:
  # Remove old movies nobody has watched (safe starter)
  old_unwatched: |
    not Watched and 
    Added < monthsAgo(4)

  # Clean up low-rated content  
  poor_quality: |
    imdbRating() < 5.5 and 
    Added < monthsAgo(2) and 
    not hasTag("keep")

  # Remove movies requested but never watched (requires Overseerr)
  unwatched_requests: |
    notWatchedByRequester() and 
    Added < daysAgo(30)

  # Free up space from large old files
  space_saver: |
    not Watched and 
    Added < monthsAgo(6) and 
    contains(Path, "2160p")

  # Clean up movies watched once and forgotten
  one_time_watches: |
    WatchCount == 1 and 
    daysSince(LastWatched) > 180
```

> **Pro Tip**: Always test with `arrbiter list` first to see what each filter matches!

---

## Advanced Filter Examples

<details>
<summary><strong>Click to expand advanced filtering scenarios</strong></summary>

### Request Accountability
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

</details>

---

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

### Import Command
The import command allows you to manually import movie files into Radarr. This is particularly useful for:
- Re-importing files from qBittorrent that need to be hardlinked
- Processing files that failed automatic import
- Importing movies from external drives or network shares

Options:
- `-p, --path`: Path to scan for importable movies (required)
- `--movie-id`: Import files for a specific movie ID only
- `--mode`: Import mode: 'move' or 'copy' (default: copy)
  - `move`: Moves files to Radarr's media folder (removes source files)
  - `copy`: Hardlinks files when possible (same filesystem), copies when not (preserves source files)
- `--auto`: Automatically import all valid files without confirmation

Example usage:
```bash
# Scan and import all valid movie files from a folder
arrbiter import --path /downloads/movies

# Import files for a specific movie only
arrbiter import --path /downloads --movie-id 123

# Use copy mode to hardlink (same filesystem) or copy (different filesystem)
arrbiter import --path /mnt/external --mode copy

# Auto-approve all valid imports
arrbiter import --path /downloads --auto
```

The command will:
1. Scan the specified folder for movie files
2. Match them against your Radarr library
3. Show quality and rejection information
4. Prompt for confirmation (unless --auto is used)
5. Import the files using Radarr's manual import API

## Tautulli Integration

When Tautulli is enabled, the tool will check if movies have been watched before deletion. This helps prevent accidentally deleting movies that users have already viewed.

### Configuration

Add Tautulli settings to your `config.yaml` as shown in the [Configuration](#configuration) section above.

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

Add Overseerr settings to your `config.yaml` as shown in the [Configuration](#configuration) section above.

### How It Works

1. The tool queries Overseerr for all movie requests via the API
2. Movies are matched by TMDB ID
3. Request information includes who requested, when, status, and who approved
4. The most recent request is used if multiple exist for the same movie

## Upgrade Movies to Better Custom Formats

The upgrade command helps ensure your library meets quality standards by finding movies that don't have your preferred custom formats and triggering upgrade searches in Radarr.

### Configuration

Configure your target custom formats in `config.yaml`:

```yaml
upgrade:
  # Custom formats to look for when upgrading movies
  custom_formats:
    - "HD Bluray Tier 01"
    - "HD Bluray Tier 02"
    - "HD Bluray Tier 03"
  
  # Match mode for custom formats:
  # - "all": Movie must have ALL specified custom formats
  # - "any": Movie must have at least ONE of the specified custom formats
  match_mode: all
  
  # Automatically monitor upgraded movies
  auto_monitor: true
```

### Usage

```bash
# Interactive mode - prompts for how many movies to upgrade
$ arrbiter upgrade

Found 12 movies missing custom formats:

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
MOVIE                                              YEAR            CURRENT FORMATS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
The Matrix                                         1999            WEB 720p
Inception                                          2010            None
The Dark Knight                                    2008            HDTV 1080p
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

How many movies would you like to upgrade? [0-12]: 3

# Unattended mode - automatically upgrade N movies
$ arrbiter upgrade --unattended 5

# Override match mode
$ arrbiter upgrade --match any  # Find movies missing ANY format
$ arrbiter upgrade --match all  # Find movies missing ALL formats (default)

# Don't monitor movies after upgrade search
$ arrbiter upgrade --no-monitor
```

### Command Options

- `--unattended N`: Run without prompts, upgrading N movies
- `--match any|all`: Override the match mode from config
- `--no-monitor`: Don't enable monitoring for upgraded movies

The command will:
1. Scan your library for movies missing the configured custom formats
2. Display a list of upgrade candidates with their current formats
3. Let you choose how many to upgrade (or use --unattended)
4. Randomly select movies if upgrading less than the total found
5. Enable monitoring if configured and movie isn't already monitored
6. Trigger Radarr searches in batches to find better versions

## Hardlink Management

The `hardlink` command helps ensure proper hardlinking between Radarr and qBittorrent, saving disk space and maintaining seeding capability.

### Features

- **Hardlink Detection**: Scans your Radarr library for movies that don't have hardlinks
- **qBittorrent Integration**: Checks if non-hardlinked movies exist in qBittorrent
- **Smart Re-importing**: Re-imports movies from qBittorrent to create proper hardlinks
- **Alternate Torrents**: Suggests other qBittorrent torrents when the original release is missing
- **Cleanup Options**: Deletes and re-searches for movies not found in qBittorrent

### Configuration

Add qBittorrent settings to your `config.yaml` as shown in the [Configuration](#configuration) section above.

### Usage

The hardlink command processes movies interactively, showing you each non-hardlinked movie and letting you decide what to do:

```bash
$ arrbiter hardlink

Scanning for non-hardlinked movies...
Found 3 non-hardlinked movies.

[1/3] The Matrix (1999)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Path: /movies/The Matrix (1999)/The.Matrix.1999.1080p.mkv
Size: 8.2 GB
Hardlinks: 1 (not hardlinked)
Status: ✓ Found in qBittorrent (actively seeding)

→ Re-import from qBittorrent to create hardlink? [y/n/q]: y
✓ Re-imported successfully

[2/3] Inception (2010)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Path: /movies/Inception (2010)/Inception.2010.1080p.mkv
Size: 12.1 GB
Hardlinks: 1 (not hardlinked)
Status: ✗ Not found in qBittorrent

→ Delete file and search for new version? [y/n/q]: n
⊘ Skipped

[3/3] Dune (2021)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Path: /movies/Dune (2021)/Dune.2021.1080p.mkv
Size: 14.4 GB
Hardlinks: 1 (not hardlinked)
Status: △ Alternate torrents available in qBittorrent
  [1] Dune.2021.2160p.HDR.x265-group2
      Score 88% | Progress 100.0% | Complete | Year match | Size 24.0 GB (+66.7% vs library)
  [2] Dune.2021.1080p.BluRay.x264-group3
      Score 74% | Progress 100.0% | Complete | Size 11.0 GB (~same size)

→ Choose alternate torrent to re-import [1-2/n/q]: 1
✓ Re-imported using "Dune.2021.2160p.HDR.x265-group2"
```

### Alternate Torrent Selection

If the original torrent path no longer exists in qBittorrent, arrbiter now scores and displays alternate torrents that match the movie title. Pick one of the numbered options to re-import from that torrent (or skip). With `--no-confirm` the top-ranked torrent is used automatically. Alternate matching prefers completed, seeding torrents with similar names, years, and sizes, so you can replace non-hardlinked files without redownloading them.

### How It Works

1. **Detection**: Uses system calls to check the hardlink count of each movie file
2. **qBittorrent Search**: For non-hardlinked files, searches qBittorrent for matching torrents
3. **Alternate Suggestions**: If the original torrent is gone, ranks other torrents in qBittorrent that match the movie
4. **Re-import**: If found in qBittorrent, uses Radarr's manual import to create a hardlink
5. **Cleanup**: If not found, optionally deletes the file and triggers a new search in Radarr

### Requirements

- Unix-like system (Linux, macOS, FreeBSD) - Windows not supported
- qBittorrent with Web UI enabled
- Radarr and qBittorrent must share the same filesystem for hardlinks to work

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

## FAQ

<details>
<summary><strong>Common Questions</strong></summary>

**Q: Is it safe to use this tool?**  
A: Yes! The tool has multiple safety features: dry-run mode is enabled by default, confirmation prompts, and detailed logging. Always test with `arrbiter list` first.

**Q: Can I undo deletions?**  
A: No, deletions are permanent. That's why dry-run mode is so important. Test your filters thoroughly before setting `dry_run: false` in your config.

**Q: Do I need all the integrations (Tautulli, Overseerr, qBittorrent)?**  
A: No! You only need Radarr. The other integrations add features but are completely optional.

**Q: How do I find my Radarr API key?**  
A: In Radarr, go to Settings > General > Security > API Key

**Q: Can I run this on a schedule?**  
A: Yes! Use `--no-confirm` flag and add it to cron/systemd. Always test your filters first.

**Q: Why are my filters not matching anything?**  
A: Check the [troubleshooting section](#-troubleshooting) below. Common issues are incorrect API keys or filter syntax.

</details>

---

## Troubleshooting

### Connection Issues

**"Failed to connect to Radarr"**
- Verify Radarr URL is correct and accessible
- Check API key in Radarr Settings > General > Security
- Ensure Radarr is running and responding

**"No movies found"**
- Check if Radarr has movies in the library
- Verify API permissions (should have read access)

### Filter Issues

**"No movies match filters"**
- Test individual filter components: `Added < monthsAgo(3)`
- Check if properties exist: some movies may not have ratings
- Use `arrbiter list` to debug what's being matched

**"Syntax error in filter"**
- Check parentheses are balanced
- Verify function names are correct (case-sensitive)
- Use proper YAML multi-line syntax with `|`

### Integration Issues

**Tautulli integration not working**
- Verify API key and URL
- Check that Tautulli has view history for your movies
- Movies are matched by IMDB ID or title

**Overseerr requests not found**
- Ensure movies were actually requested through Overseerr
- API key needs read access to requests
- Only TMDB-matched movies will have request data

### Performance Issues

**Slow startup**
- Normal for large libraries (>10k movies)
- Tautulli integration adds processing time
- Consider running with specific filters only

---

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

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

---

## License

MIT
