package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env set",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env not set",
			key:          "NONEXISTENT_KEY",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
		{
			name:         "empty env",
			key:          "EMPTY_KEY",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnv_Defaults(t *testing.T) {
	defaults := map[string]string{
		"DB_HOST":     "localhost",
		"DB_PORT":     "5432",
		"DB_NAME":     "lucendex",
		"DB_USER":     "api_ro",
		"API_PORT":    "8080",
		"DB_PASSWORD": "",
	}

	for key, defaultVal := range defaults {
		os.Unsetenv(key)
		got := getEnv(key, defaultVal)
		if got != defaultVal {
			t.Errorf("getEnv(%s) = %v, want %v", key, got, defaultVal)
		}
	}
}
