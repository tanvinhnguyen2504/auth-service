package auth

import "time"

// User mirrors the users table, minus google_sub and hd: nothing in Go reads
// them back.
type User struct {
	ID            int64     `db:"id"`
	Email         string    `db:"email"`
	EmailVerified bool      `db:"email_verified"`
	Name          string    `db:"name"`
	AvatarURL     string    `db:"avatar_url"`
	CreatedAt     time.Time `db:"created_at"`
	LastLoginAt   time.Time `db:"last_login_at"`
}

// GoogleIdentityParams carries ID token claims into the upsert. Every field
// comes from a JWKS-verified token, never from the frontend.
type GoogleIdentityParams struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
	HD            string
}

// Session is a server-side login. The id is the cookie value.
type Session struct {
	ID         string    `db:"id"`
	UserID     int64     `db:"user_id"`
	CreatedAt  time.Time `db:"created_at"`
	LastSeenAt time.Time `db:"last_seen_at"`
}

// LoginTransaction is one in-flight Google login, sealed into the oauth_tx
// cookie. Short json tags keep the ciphertext small.
type LoginTransaction struct {
	State        string    `json:"s"`
	Nonce        string    `json:"n"`
	CodeVerifier string    `json:"v"`
	ExpiresAt    time.Time `json:"e"`
}
