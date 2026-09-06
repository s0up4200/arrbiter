package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func TestConfiguredSonarrFailureStopsRun(t *testing.T) {
	oldFile, oldConfig, oldLogger, oldLevel := cfgFile, cfg, logger, zerolog.GlobalLevel()
	t.Cleanup(func() {
		cfgFile, cfg, logger = oldFile, oldConfig, oldLogger
		zerolog.SetGlobalLevel(oldLevel)
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Sonarr unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	cfgFile = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte(fmt.Sprintf("sonarr:\n  url: %s\n  api_key: test-key\n", server.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	err := initializeApp(&cobra.Command{}, nil)
	if err == nil || !strings.Contains(err.Error(), "initialize Sonarr") {
		t.Fatalf("got %v, want Sonarr initialization error", err)
	}
}
