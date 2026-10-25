package config

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

//go:embed development.yaml staging.yaml production.yaml
var configFS embed.FS

const defaultEnv = "development"

// Load reads the embedded config for APP_ENV and overlays environment variables.
func Load() (*Config, error) {
	env := envOr("APP_ENV", defaultEnv)

	raw, err := configFS.ReadFile(env + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("config: no embedded config for APP_ENV=%q", env)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("config: read %s.yaml: %w", env, err)
	}

	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("config: unmarshal %s.yaml: %w", env, err)
	}
	config.Env = env

	// PORT (how docker-compose pins the port) wins over the YAML value.
	if port := os.Getenv("PORT"); port != "" {
		config.Server.Port = strings.TrimPrefix(port, ":")
	}

	config.Database.URL = os.Getenv("DATABASE_URL")
	config.Database.Host = envOr("DB_HOST", "localhost")
	config.Database.Port = envIntOr("DB_PORT", 5432)
	config.Database.User = envOr("DB_USER", "postgres")
	config.Database.Password = os.Getenv("DB_PASSWORD")
	config.Database.Name = os.Getenv("DB_NAME")
	config.Database.SSLMode = envOr("DB_SSLMODE", "disable")

	config.Auth.GoogleOAuthClientID = os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	config.Auth.GoogleOAuthClientSecret = os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")

	config.BaseURL = strings.TrimSuffix(envOr("BASE_URL", "http://localhost:9999"), "/")
	config.FrontendURL = strings.TrimSuffix(envOr("FRONTEND_URL", "http://localhost:5173"), "/")

	config.Auth.EncryptionKey = os.Getenv("ENCRYPTION_KEY")
	config.Auth.EncryptionKeyVersion = envIntOr("ENCRYPTION_KEY_VERSION", 1)

	if config.Auth.SessionTTL, err = durationOr("SESSION_TTL", 12*time.Hour); err != nil {
		return nil, err
	}
	if config.Auth.SessionIdleTTL, err = durationOr("SESSION_IDLE_TTL", 30*time.Minute); err != nil {
		return nil, err
	}
	if config.Auth.SessionIdleTTL > config.Auth.SessionTTL {
		return nil, fmt.Errorf("config: SESSION_IDLE_TTL (%s) must not exceed SESSION_TTL (%s)",
			config.Auth.SessionIdleTTL, config.Auth.SessionTTL)
	}

	// Fail fast rather than 302 to Google with an empty client id and get an
	// opaque error back.
	for name, value := range map[string]string{
		"GOOGLE_OAUTH_CLIENT_ID":     config.Auth.GoogleOAuthClientID,
		"GOOGLE_OAUTH_CLIENT_SECRET": config.Auth.GoogleOAuthClientSecret,
		"google_oauth_redirect_url":  config.Auth.GoogleOAuthRedirectURL,
		"ENCRYPTION_KEY":             config.Auth.EncryptionKey,
	} {
		if value == "" {
			return nil, fmt.Errorf("config: %s is required", name)
		}
	}

	return config, nil
}

func durationOr(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s is not a duration (e.g. 12h, 30m): %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("config: %s must be positive, got %s", key, d)
	}
	return d, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
