package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

type Config struct {
	Env         string
	Image       string
	BaseURL     string
	FrontendURL string

	Logger struct {
		Level       string `mapstructure:"level"`
		PrettyPrint bool   `mapstructure:"pretty_print"`
		FileName    string `mapstructure:"file_name"`
		MaxSizeInMB int    `mapstructure:"max_size_in_mb"`
		MaxBackUp   int    `mapstructure:"max_back_up"`
		MaxAgeInDay int    `mapstructure:"max_age_in_day"`
	} `mapstructure:"logger"`
	Server struct {
		Port                      string `mapstructure:"port"`
		GracefulShutdownInSecond  int    `mapstructure:"graceful_shutdown_in_second"`
		ReadTimeoutInSecond       int    `mapstructure:"read_timeout_in_second"`
		ReadHeaderTimeoutInSecond int    `mapstructure:"read_header_timeout_in_second"`
		WriteTimeoutInSecond      int    `mapstructure:"write_timeout_in_second"`
		IdleTimeoutInSecond       int    `mapstructure:"idle_timeout_in_second"`
	} `mapstructure:"server"`
	Database struct {
		URL      string
		Host     string
		Port     int
		Name     string
		User     string
		Password string
		SSLMode  string

		MaxConns                int32 `mapstructure:"max_conns"`
		MinConns                int32 `mapstructure:"min_conns"`
		MaxConnLifetimeInMinute int   `mapstructure:"max_conn_lifetime_in_minute"`
		MaxConnIdleTimeInMinute int   `mapstructure:"max_conn_idle_time_in_minute"`
		ConnectTimeoutInSecond  int   `mapstructure:"connect_timeout_in_second"`
	} `mapstructure:"database"`
	Auth struct {
		GoogleOAuthRedirectURL  string `mapstructure:"google_oauth_redirect_url"`
		GoogleOAuthClientID     string
		GoogleOAuthClientSecret string
		// SessionTTL is absolute, from session creation; SessionIdleTTL resets on
		// each authenticated request. A session dies at whichever comes first.
		SessionTTL     time.Duration
		SessionIdleTTL time.Duration

		// EncryptionKey is the base64 AES-256 master key sealing the oauth_tx
		// cookie, and stored API keys in M2. EncryptionKeyVersion is stamped into
		// every sealed value so the key can be rotated.
		EncryptionKey        string
		EncryptionKeyVersion int
	} `mapstructure:"auth"`

	// Apps is the set of mini-apps this backend signs users in for. Every call
	// names one in the app-id header, so an unknown id is rejected at the edge
	// instead of reaching a handler.
	Apps struct {
		AllowedIDs []string
	}
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// ServerPort is the address the HTTP server listens on.
func (c *Config) ServerPort() string {
	return ":" + c.Server.Port
}

func (c *Config) ConnectDatabaseString() string {
	// DATABASE_URL wins when set: it is the single-line form hosting providers
	// hand you, and the documented way in. Without this the DB_* parts below
	// silently win and a correct DATABASE_URL connects nowhere.
	if c.Database.URL != "" {
		return c.Database.URL
	}

	sslMode := c.Database.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	// Built through url.URL so a password with @ or / is escaped for us.
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.Database.User, c.Database.Password),
		Host:     net.JoinHostPort(c.Database.Host, strconv.Itoa(c.Database.Port)),
		Path:     "/" + c.Database.Name,
		RawQuery: url.Values{"sslmode": {sslMode}}.Encode(),
	}

	return dsn.String()
}

// RedactedDatabaseTarget is safe to log: host, port and database name only —
// never the password, which logging the full DSN would expose.
func (c *Config) RedactedDatabaseTarget() string {
	if c.Database.URL == "" {
		return fmt.Sprintf("%s/%s", net.JoinHostPort(c.Database.Host, strconv.Itoa(c.Database.Port)), c.Database.Name)
	}
	u, err := url.Parse(c.Database.URL)
	if err != nil {
		return "(from DATABASE_URL)"
	}
	return u.Host + u.Path
}

func (c *Config) MaxConnLifetime() time.Duration {
	return time.Duration(c.Database.MaxConnLifetimeInMinute) * time.Minute
}

func (c *Config) MaxConnIdleTime() time.Duration {
	return time.Duration(c.Database.MaxConnIdleTimeInMinute) * time.Minute
}

func (c *Config) ConnectTimeout() time.Duration {
	return time.Duration(c.Database.ConnectTimeoutInSecond) * time.Second
}
