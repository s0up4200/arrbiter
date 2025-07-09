package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/s0up4200/arrbiter/radarr"
)

// Evaluate implements the Evaluate method for FieldFilter
func (f *FieldFilter) Evaluate(movie interface{}) bool {
	info, ok := movie.(radarr.MovieInfo)
	if !ok {
		return false
	}
	
	switch f.Field {
	case FieldTag:
		tagValue, ok := f.Value.(string)
		if !ok {
			return false
		}
		
		// Check if the movie has the specified tag
		for _, tag := range info.TagNames {
			matches := strings.EqualFold(tag, tagValue)
			if f.Operator == OpEquals && matches {
				return true
			}
			if f.Operator == OpNotEquals && matches {
				return false
			}
		}
		// If we didn't find the tag
		return f.Operator == OpNotEquals
		
	case FieldAddedBefore:
		dateValue, ok := f.Value.(time.Time)
		if !ok {
			return false
		}
		result := info.Added.Before(dateValue)
		if f.Operator == OpNotEquals {
			return !result
		}
		return result
		
	case FieldAddedAfter:
		dateValue, ok := f.Value.(time.Time)
		if !ok {
			return false
		}
		result := info.Added.After(dateValue)
		if f.Operator == OpNotEquals {
			return !result
		}
		return result
		
	case FieldImportedBefore:
		dateValue, ok := f.Value.(time.Time)
		if !ok {
			return false
		}
		// If no file imported date, return false for "before" checks
		if info.FileImported.IsZero() {
			return f.Operator == OpNotEquals
		}
		result := info.FileImported.Before(dateValue)
		if f.Operator == OpNotEquals {
			return !result
		}
		return result
		
	case FieldImportedAfter:
		dateValue, ok := f.Value.(time.Time)
		if !ok {
			return false
		}
		// If no file imported date, return false for "after" checks
		if info.FileImported.IsZero() {
			return f.Operator == OpNotEquals
		}
		result := info.FileImported.After(dateValue)
		if f.Operator == OpNotEquals {
			return !result
		}
		return result
		
	case FieldWatched:
		boolValue, ok := f.Value.(bool)
		if !ok {
			return false
		}
		result := info.Watched == boolValue
		if f.Operator == OpNotEquals {
			return !result
		}
		return result
		
	case FieldWatchCount:
		intValue, ok := f.Value.(int)
		if !ok {
			return false
		}
		return compareInt(info.WatchCount, intValue, f.Operator)
		
	case FieldWatchedBy:
		username, ok := f.Value.(string)
		if !ok {
			return false
		}
		
		// Check if user has watched this movie
		if userData, exists := info.UserWatchData[username]; exists {
			result := userData.Watched
			if f.Operator == OpNotEquals {
				return !result
			}
			return result
		}
		
		// User hasn't watched if no data exists
		return f.Operator == OpNotEquals
		
	case FieldWatchCountBy:
		username, ok := f.Value.(string)
		if !ok {
			return false
		}
		
		// Get user's watch count
		watchCount := 0
		if userData, exists := info.UserWatchData[username]; exists {
			watchCount = userData.WatchCount
		}
		
		// Use IntValue for comparison if set (from parsed expression like watch_count_by:"user">3)
		if f.IntValue > 0 || f.Operator != OpEquals {
			return compareInt(watchCount, f.IntValue, f.Operator)
		}
		
		// Default behavior: check if user has watched at all
		return watchCount > 0
	}
	
	return false
}

// compareInt performs integer comparison based on operator
func compareInt(a, b int, op Operator) bool {
	switch op {
	case OpEquals:
		return a == b
	case OpNotEquals:
		return a != b
	case OpGreaterThan:
		return a > b
	case OpLessThan:
		return a < b
	case OpGreaterEqual:
		return a >= b
	case OpLessEqual:
		return a <= b
	default:
		return false
	}
}

// ParseAndCreateFilter parses a filter expression and returns a filter function
func ParseAndCreateFilter(expression string) (func(radarr.MovieInfo) bool, error) {
	if strings.TrimSpace(expression) == "" {
		// Empty filter matches everything
		return func(radarr.MovieInfo) bool { return true }, nil
	}
	
	// Check if it's a legacy filter and convert it
	if IsLegacyFilter(expression) {
		convertedExpr, err := ConvertLegacyFilter(expression)
		if err != nil {
			return nil, fmt.Errorf("failed to convert legacy filter: %w", err)
		}
		expression = convertedExpr
	}
	
	// Use expr to compile and create the filter
	return CreateExprFilter(expression)
}