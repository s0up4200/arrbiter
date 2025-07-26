package config

// Config represents the complete configuration structure
type Config struct {
	Radarr      RadarrConfig      `mapstructure:"radarr"`
	Tautulli    TautulliConfig    `mapstructure:"tautulli"`
	Overseerr   OverseerrConfig   `mapstructure:"overseerr"`
	QBittorrent QBittorrentConfig `mapstructure:"qbittorrent"`
	Filter      FilterConfig      `mapstructure:"filter"`
	Safety      SafetyConfig      `mapstructure:"safety"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Upgrade     UpgradeConfig     `mapstructure:"upgrade"`
}

// RadarrConfig holds Radarr API connection details
type RadarrConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

// FilterConfig contains filter definitions
type FilterConfig map[string]string

// SafetyConfig contains safety-related settings
type SafetyConfig struct {
	DryRun        bool `mapstructure:"dry_run"`
	ConfirmDelete bool `mapstructure:"confirm_delete"`
	ShowDetails   bool `mapstructure:"show_details"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level string `mapstructure:"level"`
	Color bool   `mapstructure:"color"`
}

// TautulliConfig holds Tautulli API connection details and watch settings
type TautulliConfig struct {
	URL             string  `mapstructure:"url"`
	APIKey          string  `mapstructure:"api_key"`
	MinWatchPercent float64 `mapstructure:"min_watch_percent"`
}

// OverseerrConfig holds Overseerr API connection details
type OverseerrConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

// QBittorrentConfig holds qBittorrent API connection details
type QBittorrentConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// UpgradeConfig holds movie upgrade configuration
type UpgradeConfig struct {
	CustomFormats []string `mapstructure:"custom_formats"`
	MatchMode     string   `mapstructure:"match_mode"`
	AutoMonitor   bool     `mapstructure:"auto_monitor"`
}
