package config

import (
	"fmt"
	"os"
	"path/filepath"

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

		// Check home directory
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".radarr-cleanup"))
		}

		// Check /etc
		v.AddConfigPath("/etc/radarr-cleanup/")
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found: %w", err)
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

	// Safety defaults
	v.SetDefault("safety.dry_run", true)
	v.SetDefault("safety.confirm_delete", true)
	v.SetDefault("safety.show_details", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")
	v.SetDefault("logging.color", true)
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

	// Validate logging format
	validFormats := map[string]bool{
		"console": true,
		"json":    true,
	}
	if !validFormats[cfg.Logging.Format] {
		return fmt.Errorf("invalid logging format: %s", cfg.Logging.Format)
	}

	return nil
}
