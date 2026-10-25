package config

import "testing"

func TestConnectDatabaseString(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*Config)
		want string
	}{
		{
			name: "DATABASE_URL wins when set",
			cfg: func(c *Config) {
				c.Database.URL = "postgres://u:p@db.example.com:5432/app?sslmode=require"
				// Deliberately also set the parts, to prove the URL takes precedence.
				c.Database.Host = "localhost"
				c.Database.Port = 5432
				c.Database.Name = "ignored"
			},
			want: "postgres://u:p@db.example.com:5432/app?sslmode=require",
		},
		{
			name: "falls back to the DB_* parts",
			cfg: func(c *Config) {
				c.Database.Host = "localhost"
				c.Database.Port = 5432
				c.Database.User = "postgres"
				c.Database.Password = "postgres"
				c.Database.Name = "auth"
				c.Database.SSLMode = "disable"
			},
			want: "postgres://postgres:postgres@localhost:5432/auth?sslmode=disable",
		},
		{
			name: "an empty sslmode defaults to disable",
			cfg: func(c *Config) {
				c.Database.Host = "localhost"
				c.Database.Port = 5432
				c.Database.User = "postgres"
				c.Database.Name = "auth"
			},
			// url.UserPassword renders the colon even for an empty password.
			want: "postgres://postgres:@localhost:5432/auth?sslmode=disable",
		},
		{
			// url.URL escapes the password, so a DSN-breaking character cannot
			// corrupt the connection string.
			name: "a password with reserved characters is escaped",
			cfg: func(c *Config) {
				c.Database.Host = "localhost"
				c.Database.Port = 5432
				c.Database.User = "postgres"
				c.Database.Password = "p@ss/word"
				c.Database.Name = "auth"
			},
			want: "postgres://postgres:p%40ss%2Fword@localhost:5432/auth?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			tt.cfg(&c)

			if got := c.ConnectDatabaseString(); got != tt.want {
				t.Errorf("ConnectDatabaseString() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}
