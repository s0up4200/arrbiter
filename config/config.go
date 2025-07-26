package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Load loads the configuration from file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Look for config in standard locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")

		// Check current directory first
		v.AddConfigPath(".")

		// Check XDG config directory and legacy locations
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "arrbiter"))
			v.AddConfigPath(filepath.Join(home, ".arrbiter"))
		}
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Try to create default config
			if err := createDefaultConfig(); err != nil {
				return nil, fmt.Errorf("config file not found and unable to create default: %w", err)
			}
			log.Info().Msg("Created default config file at ~/.config/arrbiter/config.yaml")
			log.Info().Msg("Please edit it with your API keys and run the command again")
			return nil, fmt.Errorf("config file created at ~/.config/arrbiter/config.yaml - please edit it with your API keys")
		}
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Radarr defaults
	v.SetDefault("radarr.url", "http://localhost:7878")

	// Tautulli defaults
	v.SetDefault("tautulli.min_watch_percent", 85.0)

	// Safety defaults
	v.SetDefault("safety.dry_run", true)
	v.SetDefault("safety.confirm_delete", true)
	v.SetDefault("safety.show_details", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.color", true)

	// Upgrade defaults
	v.SetDefault("upgrade.custom_formats", []string{})
	v.SetDefault("upgrade.match_mode", "all")
	v.SetDefault("upgrade.auto_monitor", true)
}

// validate checks if the configuration is valid
func validate(cfg *Config) error {
	if cfg.Radarr.URL == "" {
		return fmt.Errorf("radarr.url is required")
	}

	if cfg.Radarr.APIKey == "" || cfg.Radarr.APIKey == "your-api-key-here" {
		return fmt.Errorf("radarr.api_key must be set to a valid API key")
	}

	// Validate logging level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s", cfg.Logging.Level)
	}

	// Validate upgrade match mode
	if cfg.Upgrade.MatchMode != "" && cfg.Upgrade.MatchMode != "any" && cfg.Upgrade.MatchMode != "all" {
		return fmt.Errorf("invalid upgrade.match_mode: %s (must be 'any' or 'all')", cfg.Upgrade.MatchMode)
	}

	return nil
}

// createDefaultConfig creates a default config file in ~/.config/arrbiter/
func createDefaultConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unable to get home directory: %w", err)
	}

	// Create config directory
	configDir := filepath.Join(home, ".config", "arrbiter")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}

	// Path to the new config file
	configPath := filepath.Join(configDir, "config.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	// Try to find config.yaml.example in common locations
	examplePaths := []string{
		"config.yaml.example",
		"./config.yaml.example",
		filepath.Join(home, ".config", "arrbiter", "config.yaml.example"),
		"/usr/share/arrbiter/config.yaml.example",
		"/usr/local/share/arrbiter/config.yaml.example",
	}

	var sourceFile *os.File
	for _, path := range examplePaths {
		file, err := os.Open(path)
		if err == nil {
			sourceFile = file
			break
		}
	}

	if sourceFile == nil {
		// Create a minimal default config
		defaultConfig := `radarr:
  url: http://localhost:7878
  api_key: your-api-key-here

tautulli:
  url: http://localhost:8181
  api_key: your-tautulli-api-key
  min_watch_percent: 85  # Consider watched if > 85% viewed

overseerr:
  url: http://localhost:5055
  api_key: your-overseerr-api-key

qbittorrent:
  url: http://localhost:8080
  username: admin
  password: adminpass

filter:
  # Example filters - customize these for your needs
  unwatched_requests: notWatchedByRequester() and Added < daysAgo(30)
  space_cleanup: not Watched and Added < monthsAgo(3) and not hasTag("keep")
  poor_quality: imdbRating() < 5.5 and notRequested() and Added < daysAgo(30)

safety:
  dry_run: true
  confirm_delete: true
  show_details: true

logging:
  level: info
  format: console
  color: true

upgrade:
  # Custom formats to upgrade to (e.g., ["Remux-1080p", "Bluray-1080p"])
  custom_formats: []
  # Match mode: "all" (must match all formats) or "any" (match any format)
  match_mode: all
  # Automatically monitor upgraded movies
  auto_monitor: true
`
		return os.WriteFile(configPath, []byte(defaultConfig), 0644)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer destFile.Close()

	// Copy the content
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("unable to copy config content: %w", err)
	}

	return nil
}
