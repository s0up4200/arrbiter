package qbittorrent

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	minTitleMatchThreshold   = 0.6
	defaultMaxTorrentMatches = 5
)

// FindAlternateTorrents attempts to find torrents matching the provided movie metadata.
// It is used when a Radarr movie file is not hardlinked to an existing torrent.
func (c *Client) FindAlternateTorrents(ctx context.Context, title string, year int, targetSize int64) ([]*TorrentMatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	titleTokens := tokenizeTitle(title)
	if len(titleTokens) == 0 {
		return nil, nil
	}

	torrents, err := c.GetAllTorrents(ctx)
	if err != nil {
		return nil, err
	}

	matches := make([]*TorrentMatch, 0, len(torrents))

	for _, torrent := range torrents {
		if torrent == nil || torrent.Name == "" {
			continue
		}

		match := evaluateTorrentMatch(torrent, titleTokens, year, targetSize)
		if match == nil {
			continue
		}

		matches = append(matches, match)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			// Prefer closer size difference when scores tie and size is known.
			return abs64(matches[i].SizeDifference) < abs64(matches[j].SizeDifference)
		}
		return matches[i].Score > matches[j].Score
	})

	if len(matches) > defaultMaxTorrentMatches {
		matches = matches[:defaultMaxTorrentMatches]
	}

	return matches, nil
}

// evaluateTorrentMatch returns a TorrentMatch when the torrent is similar enough to the desired movie.
func evaluateTorrentMatch(torrent *TorrentInfo, desiredTokens []string, targetYear int, targetSize int64) *TorrentMatch {
	if len(desiredTokens) == 0 {
		return nil
	}

	tokens := tokenizeTitle(torrent.Name)
	if len(tokens) == 0 {
		return nil
	}

	titleMatch := computeTokenMatch(desiredTokens, tokens)
	if titleMatch < minTitleMatchThreshold {
		return nil
	}

	score := titleMatch

	candidateYears := extractYearTokens(tokens)
	conflictingYear := false
	hasTargetYear := false

	if targetYear > 0 {
		for _, yr := range candidateYears {
			if yr == targetYear {
				hasTargetYear = true
			} else {
				conflictingYear = true
			}
		}
		if conflictingYear {
			// Another distinct year in the torrent title usually implies a different movie.
			return nil
		}
		if hasTargetYear {
			score += 0.07
		} else {
			// If the target year is known and no year is present, penalize slightly.
			score -= 0.15
		}
	}

	if torrent.IsSeeding {
		score += 0.05
	}

	if torrent.IsComplete() {
		score += 0.05
	} else if torrent.Progress < 0.9 {
		// Penalize torrents that are far from completion.
		score -= 0.15
	}

	var sizeDiff int64
	if targetSize > 0 && torrent.Size > 0 {
		sizeDiff = torrent.Size - targetSize
		sizeSimilarity := 1 - math.Min(1, math.Abs(float64(sizeDiff))/float64(targetSize))
		score += sizeSimilarity * 0.2
	}

	// Clamp score to a sensible range
	if score < 0 {
		score = 0
	} else if score > 1 {
		score = 1
	}

	return &TorrentMatch{
		Torrent:        torrent,
		Score:          score,
		TitleMatch:     titleMatch,
		YearMatched:    hasTargetYear,
		SizeDifference: sizeDiff,
	}
}

// tokenizeTitle splits a title or torrent name into normalized tokens for comparison.
func tokenizeTitle(input string) []string {
	clean := normalizeTitle(input)
	if clean == "" {
		return nil
	}
	return strings.Fields(clean)
}

// normalizeTitle converts a title into a lowercase string with only alphanumeric tokens separated by spaces.
func normalizeTitle(input string) string {
	var b strings.Builder
	lastSpace := true

	for _, r := range strings.ToLower(input) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastSpace = false
		default:
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func extractYearTokens(tokens []string) []int {
	var years []int
	for _, token := range tokens {
		if len(token) != 4 {
			continue
		}
		year, err := strconv.Atoi(token)
		if err != nil {
			continue
		}
		if year >= 1900 && year <= 2100 {
			years = append(years, year)
		}
	}
	return years
}

// computeTokenMatch returns intersection proportion of desired tokens in candidate tokens.
func computeTokenMatch(desired, candidate []string) float64 {
	if len(desired) == 0 || len(candidate) == 0 {
		return 0
	}

	candidateSet := make(map[string]struct{}, len(candidate))
	for _, token := range candidate {
		candidateSet[token] = struct{}{}
	}

	var matches int
	for _, token := range desired {
		if _, ok := candidateSet[token]; ok {
			matches++
		}
	}

	return float64(matches) / float64(len(desired))
}

func abs64(val int64) int64 {
	if val < 0 {
		return -val
	}
	return val
}
