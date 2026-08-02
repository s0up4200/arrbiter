# Sonarr Integration — Design

**Date:** 2026-08-02
**Status:** Approved
**Scope:** v1 — series-level cleanup (list/delete) for TV, mirroring the existing Radarr movie flow.

## Goal

Extend arrbiter's cleanup flow to TV: find requested-but-abandoned series via filter
expressions (Tautulli watch data + Overseerr request data) and delete them from Sonarr.
Whole-series granularity only. Season trimming, upgrade, and hardlink support for TV are
explicitly out of scope for v1.

## Approach

Mirror package. New `sonarr/` package shaped like `radarr/`; no shared media abstraction
(two types don't justify it). Existing movie code is untouched.

## Config

New optional block, enabled when both `url` and `api_key` are set (same pattern as
Tautulli/Overseerr):

```yaml
sonarr:
  url: https://sonarr.example.com
  api_key: ...

filter_tv:
  abandoned: isRequested() && notWatchedByRequester() && Added < daysAgo(60)
```

- `filter_tv:` is a `map[string]string`, parallel to `filter:`. Movie filters and series
  filters never mix.
- `tautulli.min_watch_percent` is reused as the per-episode watched threshold.

## sonarr/ package

- `client.go` — wraps `golift.io/starr/sonarr` (starr v1.1.0, already a dependency).
  Fetch all series + tags in batch, map tag IDs to names.
- `SeriesInfo` struct fields exposed to filters: `Title`, `Year`, `TagNames`, `Added`,
  `EpisodeCount` (downloaded/available), `TotalEpisodes`, `Ended` (bool, show status),
  `SizeOnDisk`, `UserWatchData` (per-user), request fields (requester, date, status).
- `operations.go` — list/delete operations honoring the existing `safety:` settings
  (dry-run, confirm_delete, show_details). Delete = Sonarr series delete with file removal.
- `enrichers.go` — Tautulli + Overseerr enrichment (below).

## Watch semantics (Tautulli)

- Batch-fetch episode history once (`media_type=episode`), group in-memory by
  `grandparent_rating_key` (fallback: show title + year), per user.
- An episode counts as watched for a user when `percent_complete >= min_watch_percent`.
- Per-user, per-series data: distinct watched episode count, last watched date.
- Series-level progress for a user = distinct watched episodes / available (downloaded)
  episodes.

## Request semantics (Overseerr)

Reuse the existing Overseerr client; fetch `type == "tv"` requests and match to series by
TVDB/TMDB ID. Same request helpers as movies.

## Filter environment (series)

Second expr environment in `filter/`, compiled against `SeriesInfo`. Helper names mirror
the movie env so filters feel identical:

- Tags: `hasTag()`
- Dates: `daysAgo()`, `monthsAgo()`, `yearsAgo()`, `parseDate()`
- Watch: `watchedBy(u)` (progress ≥ min_watch_percent of available episodes),
  `watchProgressBy(u)` (0–100, % of available episodes), `watchCountBy(u)`,
  `lastWatchedBy(u)` (time, zero value when never watched)
- Requests: `requestedBy()`, `requestedAfter()`, `requestedBefore()`, `requestStatus()`,
  `approvedBy()`, `isRequested()`, `notRequested()`, `watchedByRequester()`,
  `notWatchedByRequester()`

Movie-only helpers (ratings, hardlink) are absent from the series env: a `filter_tv`
expression using them fails at compile time with a clear error, never silently.

## CLI flow

`arrbiter list` and `arrbiter delete` process movies first (unchanged), then series when
Sonarr is configured: evaluate every `filter_tv` entry, group results by filter name,
dedupe series across filters for deletion. `arrbiter test` gains a Sonarr connectivity
check. Output formatting mirrors the movie tree style.

## Error handling

- Sonarr unreachable → fail the run with a clear error (same as Radarr today).
- Tautulli/Overseerr disabled → watch/request helpers return zero values, as with movies.
- Series with no episode file on disk: `watchProgressBy` denominator is 0 → progress 0;
  `watchedBy` is false.

## Testing

- Unit tests for the series filter env (compile + evaluate against fixture `SeriesInfo`).
- Unit tests for episode-history grouping with canned Tautulli payloads (multi-user,
  partial watches, rewatches).
- Existing movie tests must pass unchanged.

## Out of scope (v2 candidates)

- Season-level trimming (delete watched seasons; `filter_tv` shape leaves room).
- TV upgrade / hardlink support.
- Overseerr request cleanup on delete.
