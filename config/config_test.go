package config

import (
	"testing"
)

func TestValidateUpgradeMatchMode(t *testing.T) {
	tests := []struct {
		name      string
		matchMode string
		wantErr   bool
	}{
		{
			name:      "Valid match mode - any",
			matchMode: "any",
			wantErr:   false,
		},
		{
			name:      "Valid match mode - all",
			matchMode: "all",
			wantErr:   false,
		},
		{
			name:      "Empty match mode (uses default)",
			matchMode: "",
			wantErr:   false,
		},
		{
			name:      "Invalid match mode",
			matchMode: "invalid",
			wantErr:   true,
		},
		{
			name:      "Invalid match mode - some",
			matchMode: "some",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Radarr: RadarrConfig{
					URL:    "http://localhost:7878",
					APIKey: "valid-api-key",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
				Upgrade: UpgradeConfig{
					MatchMode: tt.matchMode,
				},
			}

			err := validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				// Verify error message mentions upgrade.match_mode
				if err.Error() == "" || (tt.matchMode != "" && err.Error() != "invalid upgrade.match_mode: "+tt.matchMode+" (must be 'any' or 'all')") {
					t.Errorf("validate() error message = %v, want message about invalid upgrade.match_mode", err.Error())
				}
			}
		})
	}
}