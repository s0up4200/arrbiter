package qbittorrent

import "testing"

func TestNormalizeTitle(t *testing.T) {
	cases := map[string]string{
		"The.Matrix.1999.1080p.BluRay.x264-GROUP": "the matrix 1999 1080p bluray x264 group",
		"Movie Name (2023) [IMAX]":                "movie name 2023 imax",
		"":                                        "",
	}

	for input, want := range cases {
		if got := normalizeTitle(input); got != want {
			t.Fatalf("normalizeTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestComputeTokenMatch(t *testing.T) {
	desired := []string{"movie", "name", "2023"}
	candidate := []string{"movie", "name", "2023", "bluray"}

	got := computeTokenMatch(desired, candidate)
	if got != 1 {
		t.Fatalf("computeTokenMatch returned %f, want 1.0", got)
	}

	if computeTokenMatch(desired, []string{"movie"}) <= 0.3 {
		t.Fatalf("expected partial overlap to be > 0.3")
	}
}

func TestEvaluateTorrentMatch(t *testing.T) {
	desired := tokenizeTitle("Awesome Movie 2023")

	torrent := &TorrentInfo{
		Name:      "Awesome.Movie.2023.1080p.WEBRip.x265-Group",
		Progress:  1.0,
		IsSeeding: true,
		Size:      8 * 1024 * 1024 * 1024,
	}

	match := evaluateTorrentMatch(torrent, desired, 2023, 8*1024*1024*1024)
	if match == nil {
		t.Fatalf("expected torrent to match")
	}
	if !match.YearMatched {
		t.Fatalf("expected YearMatched to be true")
	}
	if match.Score < 0.7 {
		t.Fatalf("expected score to be >= 0.7, got %.2f", match.Score)
	}

	// Large mismatches should return nil.
	noMatch := evaluateTorrentMatch(torrent, tokenizeTitle("Different Film"), 2019, 4*1024*1024*1024)
	if noMatch != nil {
		t.Fatalf("expected mismatch to return nil")
	}
}

func TestEvaluateTorrentMatchRejectsConflictingYear(t *testing.T) {
	desired := tokenizeTitle("Mad Max 1979")

	torrent := &TorrentInfo{
		Name:      "Mad.Max.2.1981.1080p.BluRay.x264-GRP",
		Progress:  1.0,
		IsSeeding: true,
		Size:      7 * 1024 * 1024 * 1024,
	}

	match := evaluateTorrentMatch(torrent, desired, 1979, 7*1024*1024*1024)
	if match != nil {
		t.Fatalf("expected torrent with conflicting year to be rejected")
	}
}
