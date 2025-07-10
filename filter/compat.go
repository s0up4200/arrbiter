package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// ConvertLegacyFilter converts old filter syntax to expr syntax
func ConvertLegacyFilter(oldFilter string) (string, error) {
	// Handle empty filter
	if strings.TrimSpace(oldFilter) == "" {
		return "", nil
	}

	// First, handle logical operators
	filter := strings.ReplaceAll(oldFilter, " AND ", " and ")
	filter = strings.ReplaceAll(filter, " OR ", " or ")
	filter = strings.ReplaceAll(filter, " NOT ", " not ")

	// Regular expressions for different filter patterns
	patterns := map[*regexp.Regexp]func([]string) string{
		// tag:"value" or tag!:"value"
		regexp.MustCompile(`tag(!?):"([^"]+)"`): func(matches []string) string {
			if matches[1] == "!" {
				return fmt.Sprintf(`not hasTag("%s")`, matches[2])
			}
			return fmt.Sprintf(`hasTag("%s")`, matches[2])
		},

		// watched_by:"user" or watched_by!:"user"
		regexp.MustCompile(`watched_by(!?):"([^"]+)"`): func(matches []string) string {
			if matches[1] == "!" {
				return fmt.Sprintf(`not watchedBy("%s")`, matches[2])
			}
			return fmt.Sprintf(`watchedBy("%s")`, matches[2])
		},

		// watch_count_by:"user">N
		regexp.MustCompile(`watch_count_by:"([^"]+)"([><=]+)(\d+)`): func(matches []string) string {
			return fmt.Sprintf(`watchCountBy("%s") %s %s`, matches[1], matches[2], matches[3])
		},

		// watched:true/false
		regexp.MustCompile(`watched:(true|false)`): func(matches []string) string {
			return fmt.Sprintf(`Movie.Watched == %s`, matches[1])
		},

		// watch_count:>N
		regexp.MustCompile(`watch_count:([><=]+)(\d+)`): func(matches []string) string {
			return fmt.Sprintf(`Movie.WatchCount %s %s`, matches[1], matches[2])
		},

		// added_before:"YYYY-MM-DD"
		regexp.MustCompile(`added_before:"([^"]+)"`): func(matches []string) string {
			return fmt.Sprintf(`Movie.Added < "%s"`, matches[1])
		},

		// added_after:"YYYY-MM-DD"
		regexp.MustCompile(`added_after:"([^"]+)"`): func(matches []string) string {
			return fmt.Sprintf(`Movie.Added > "%s"`, matches[1])
		},

		// imported_before:"YYYY-MM-DD"
		regexp.MustCompile(`imported_before:"([^"]+)"`): func(matches []string) string {
			return fmt.Sprintf(`Movie.FileImported < "%s"`, matches[1])
		},

		// imported_after:"YYYY-MM-DD"
		regexp.MustCompile(`imported_after:"([^"]+)"`): func(matches []string) string {
			return fmt.Sprintf(`Movie.FileImported > "%s"`, matches[1])
		},
	}

	// Apply all pattern replacements
	for pattern, replacer := range patterns {
		filter = pattern.ReplaceAllStringFunc(filter, func(match string) string {
			matches := pattern.FindStringSubmatch(match)
			return replacer(matches)
		})
	}

	return filter, nil
}

// IsLegacyFilter checks if a filter uses the old syntax
func IsLegacyFilter(filter string) bool {
	legacyPatterns := []string{
		"tag:",
		"watched_by:",
		"watch_count_by:",
		"watched:",
		"watch_count:",
		"added_before:",
		"added_after:",
		"imported_before:",
		"imported_after:",
	}

	for _, pattern := range legacyPatterns {
		if strings.Contains(filter, pattern) {
			return true
		}
	}

	return false
}
