package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/s0up4200/arrbiter/filter"
	"github.com/s0up4200/arrbiter/sonarr"
)

// tvFiltersEnabled reports whether series processing should run.
func tvFiltersEnabled() bool {
	return sonarrOperations != nil && len(cfg.FilterTV) > 0
}

// movieFiltersEnabled reports whether movie processing should run.
func movieFiltersEnabled() bool {
	return operations != nil && len(cfg.Filter) > 0
}

// requireRadarr errors when a command needs Radarr but it is not configured.
func requireRadarr() error {
	if operations == nil {
		return fmt.Errorf("Radarr is not configured (required for this command)")
	}
	return nil
}

// evaluateSeriesFilters fetches all series and evaluates every filter_tv entry.
func evaluateSeriesFilters(ctx context.Context) (map[string][]sonarr.SeriesInfo, map[int64]sonarr.SeriesInfo, error) {
	allSeries, err := sonarrOperations.GetAllSeries(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get series: %w", err)
	}

	seriesByFilter := make(map[string][]sonarr.SeriesInfo)
	uniqueSeries := make(map[int64]sonarr.SeriesInfo)

	for filterName, filterExpr := range cfg.FilterTV {
		logger.Debug().Str("filter", filterName).Str("expression", filterExpr).Msg("Processing TV filter")

		filterFunc, err := filter.ParseAndCreateSeriesFilter(filterExpr)
		if err != nil {
			logger.Error().Err(err).Str("filter", filterName).Msg("Invalid TV filter expression")
			continue
		}

		for _, series := range allSeries {
			if filterFunc(series) {
				seriesByFilter[filterName] = append(seriesByFilter[filterName], series)
				uniqueSeries[series.ID] = series
			}
		}
	}

	return seriesByFilter, uniqueSeries, nil
}

// listSeries evaluates TV filters and prints matching series.
func listSeries(ctx context.Context) error {
	logger.Info().Int("filter_count", len(cfg.FilterTV)).Msg("Processing TV filters")

	seriesByFilter, uniqueSeries, err := evaluateSeriesFilters(ctx)
	if err != nil {
		return err
	}

	if len(uniqueSeries) == 0 {
		fmt.Println("No series found matching any TV filter criteria.")
		return nil
	}

	fmt.Printf("\nFound %d series\n\n", len(uniqueSeries))
	printSeriesByFilter(seriesByFilter)
	return nil
}

// deleteSeries evaluates TV filters and deletes matching series.
func deleteSeries(ctx context.Context) error {
	logger.Info().Int("filter_count", len(cfg.FilterTV)).Msg("Processing TV filters for deletion")

	seriesByFilter, uniqueSeries, err := evaluateSeriesFilters(ctx)
	if err != nil {
		return err
	}

	if len(uniqueSeries) == 0 {
		fmt.Println("No series found matching any TV filter criteria.")
		return nil
	}

	fmt.Printf("\nFound %d series to delete\n\n", len(uniqueSeries))
	printSeriesByFilter(seriesByFilter)

	seriesToDelete := make([]sonarr.SeriesInfo, 0, len(uniqueSeries))
	for _, s := range uniqueSeries {
		seriesToDelete = append(seriesToDelete, s)
	}

	deleteOpts := sonarr.DeleteOptions{
		DryRun:        cfg.Safety.DryRun,
		ConfirmDelete: cfg.Safety.ConfirmDelete && !noConfirm,
	}

	return sonarrOperations.DeleteSeries(ctx, seriesToDelete, deleteOpts)
}

// printSeriesByFilter prints series grouped by filter in the movie tree style.
func printSeriesByFilter(seriesByFilter map[string][]sonarr.SeriesInfo) {
	for filterName, series := range seriesByFilter {
		if len(series) == 0 {
			continue
		}

		fmt.Printf("╭─ TV Filter: %s (%d match", filterName, len(series))
		if len(series) != 1 {
			fmt.Printf("es")
		}
		fmt.Println(")")

		for i, s := range series {
			isLast := i == len(series)-1
			prefix := "├"
			if isLast {
				prefix = "╰"
			}

			fmt.Printf("%s── %s (%d)\n", prefix, s.Title, s.Year)
			if cfg.Safety.ShowDetails {
				indent := "│   "
				if isLast {
					indent = "    "
				}

				if len(s.TagNames) > 0 {
					fmt.Printf("%sTags: %s\n", indent, strings.Join(s.TagNames, ", "))
				}

				status := s.Status
				if s.Ended {
					status = "ended"
				}
				fmt.Printf("%sEpisodes on disk: %d/%d | Status: %s | Added: %s\n",
					indent, s.EpisodeCount, s.TotalEpisodes, status, s.Added.Format("2006-01-02"))

				if s.WatchCount > 0 {
					watchInfo := fmt.Sprintf("Watched: %d episodes (%d plays)", s.WatchedEpisodes, s.WatchCount)
					if !s.LastWatched.IsZero() {
						watchInfo += fmt.Sprintf(" (last: %s)", s.LastWatched.Format("2006-01-02"))
					}
					fmt.Printf("%s%s\n", indent, watchInfo)
				}

				if s.IsRequested && s.RequestedBy != "" {
					requestInfo := fmt.Sprintf("Requested by: %s", s.RequestedBy)
					if !s.RequestDate.IsZero() {
						requestInfo += fmt.Sprintf(" on %s", s.RequestDate.Format("2006-01-02"))
					}
					fmt.Printf("%s%s\n", indent, requestInfo)
				}
			}
			if i < len(series)-1 && cfg.Safety.ShowDetails {
				fmt.Printf("│\n")
			}
		}
		fmt.Println()
	}
}
