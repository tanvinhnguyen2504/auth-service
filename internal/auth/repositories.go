package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoSession = errors.New("auth: no session")
	ErrNoUser    = errors.New("auth: no user")
)

const userColumns = "id, email, email_verified, name, avatar_url, created_at, last_login_at"

type Repository interface {
	UpsertGoogleIdentity(ctx context.Context, p GoogleIdentityParams) (*User, bool, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)

	CreateSession(ctx context.Context, s Session) error
	// GetSession returns a session only if it is within both TTLs. The cutoffs
	// are passed in so the SQL decides what is still alive.
	GetSession(ctx context.Context, id string, absoluteCutoff, idleCutoff time.Time) (*Session, error)
	UpdateSessionLastSeenAt(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID int64) error
	DeleteExpiredSessions(ctx context.Context, absoluteCutoff, idleCutoff time.Time) (int64, error)
}

type repository struct {
	pgPool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) Repository {
	return &repository{pgPool: pool}
}

func (r *repository) UpsertGoogleIdentity(ctx context.Context, p GoogleIdentityParams) (*User, bool, error) {
	const query = `
INSERT INTO users (google_sub, email, email_verified, name, avatar_url, hd)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (google_sub) DO UPDATE SET
    email          = $2,
    email_verified = $3,
    name           = $4,
    avatar_url     = $5,
    hd             = $6,
    last_login_at  = now()
RETURNING ` + userColumns + `, (xmax = 0) AS is_new_user`

	// Scanned explicitly: is_new_user has no field on User, so RowToStructByName
	// would reject the extra column. Keep this list in userColumns order.
	var u User
	var isNewUser bool
	err := r.pgPool.QueryRow(ctx, query,
		p.Subject, p.Email, p.EmailVerified, p.Name, p.AvatarURL, p.HD,
	).Scan(&u.ID, &u.Email, &u.EmailVerified, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.LastLoginAt, &isNewUser)
	if err != nil {
		return nil, false, fmt.Errorf("auth: upsert google user: %w", err)
	}

	return &u, isNewUser, nil
}

func (r *repository) GetUserByID(ctx context.Context, id int64) (*User, error) {
	const query = `SELECT ` + userColumns + ` FROM users WHERE id = $1`

	user, err := queryOne[User](ctx, r.pgPool, query, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoUser
	}
	if err != nil {
		return nil, fmt.Errorf("auth: user by id: %w", err)
	}
	return user, nil
}

func (r *repository) CreateSession(ctx context.Context, s Session) error {
	const query = `INSERT INTO sessions (id, user_id) VALUES ($1, $2)`

	if _, err := r.pgPool.Exec(ctx, query, s.ID, s.UserID); err != nil {
		return fmt.Errorf("auth: create session: %w", err)
	}
	return nil
}

func (r *repository) GetSession(ctx context.Context, id string, absoluteCutoff, idleCutoff time.Time) (*Session, error) {
	const query = `
SELECT id, user_id, created_at, last_seen_at
FROM sessions
WHERE id = $1 AND created_at > $2 AND last_seen_at > $3`

	session, err := queryOne[Session](ctx, r.pgPool, query, id, absoluteCutoff, idleCutoff)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get session: %w", err)
	}
	return session, nil
}

func (r *repository) UpdateSessionLastSeenAt(ctx context.Context, id string) error {
	const query = `UPDATE sessions SET last_seen_at = now() WHERE id = $1`

	if _, err := r.pgPool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("auth: update session last_seen_at: %w", err)
	}
	return nil
}

func (r *repository) DeleteSession(ctx context.Context, id string) error {
	const query = `DELETE FROM sessions WHERE id = $1`

	if _, err := r.pgPool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

func (r *repository) DeleteUserSessions(ctx context.Context, userID int64) error {
	const query = `DELETE FROM sessions WHERE user_id = $1`

	if _, err := r.pgPool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("auth: delete user sessions: %w", err)
	}
	return nil
}

func (r *repository) DeleteExpiredSessions(ctx context.Context, absoluteCutoff, idleCutoff time.Time) (int64, error) {
	const query = `DELETE FROM sessions WHERE created_at <= $1 OR last_seen_at <= $2`

	tag, err := r.pgPool.Exec(ctx, query, absoluteCutoff, idleCutoff)
	if err != nil {
		return 0, fmt.Errorf("auth: delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// queryOne maps one row onto T by column name. It returns the raw error so a
// caller can match pgx.ErrNoRows; wrapping is the caller's job.
func queryOne[T any](ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (*T, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, err
	}
	return &row, nil
}
