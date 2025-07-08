package config

// Config represents the complete configuration structure
type Config struct {
	Radarr   RadarrConfig   `mapstructure:"radarr"`
	Tautulli TautulliConfig `mapstructure:"tautulli"`
	Filter   FilterConfig   `mapstructure:"filter"`
	Safety   SafetyConfig   `mapstructure:"safety"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// RadarrConfig holds Radarr API connection details
type RadarrConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

// FilterConfig contains filter settings and presets
type FilterConfig struct {
	DefaultExpression string                  `mapstructure:"default_expression"`
	Presets           map[string]FilterPreset `mapstructure:"presets"`
}

// FilterPreset represents a named filter configuration
type FilterPreset struct {
	Expression string `mapstructure:"expression"`
}

// SafetyConfig contains safety-related settings
type SafetyConfig struct {
	DryRun        bool `mapstructure:"dry_run"`
	ConfirmDelete bool `mapstructure:"confirm_delete"`
	ShowDetails   bool `mapstructure:"show_details"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Color  bool   `mapstructure:"color"`
}

// TautulliConfig holds Tautulli API connection details and watch settings
type TautulliConfig struct {
	Enabled    bool             `mapstructure:"enabled"`
	URL        string           `mapstructure:"url"`
	APIKey     string           `mapstructure:"api_key"`
	WatchCheck WatchCheckConfig `mapstructure:"watch_check"`
}

// WatchCheckConfig contains settings for watch status checking
type WatchCheckConfig struct {
	Enabled         bool    `mapstructure:"enabled"`
	MinWatchPercent float64 `mapstructure:"min_watch_percent"`
}
