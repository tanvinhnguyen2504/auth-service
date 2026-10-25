package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/nguyentanvinh/auth-be/backend/internal/secret"
)

// loginTxTTL bounds how long a user may sit on Google's consent screen.
const loginTxTTL = 10 * time.Minute

type Service interface {
	StartGoogleLogin(ctx context.Context) (LoginRedirect, error)
	CompleteLogin(ctx context.Context, in CallbackInput) (LoginResult, error)
	SessionUser(ctx context.Context, sessionID string) (*User, error)
	Logout(ctx context.Context, sessionID string) error
}

type service struct {
	google IdentityProvider
	repo   Repository
	sealer *secret.Box
	ttl    time.Duration
	idle   time.Duration
}

func NewService(google IdentityProvider, repo Repository, sealer *secret.Box, sessionTTL, sessionIdleTTL time.Duration) Service {
	return &service{google: google, repo: repo, sealer: sealer, ttl: sessionTTL, idle: sessionIdleTTL}
}

func (s *service) StartGoogleLogin(ctx context.Context) (LoginRedirect, error) {
	state, err := randomToken()
	if err != nil {
		return LoginRedirect{}, err
	}
	nonce, err := randomToken()
	if err != nil {
		return LoginRedirect{}, err
	}

	codeVerifier := oauth2.GenerateVerifier()

	oauthTx, err := sealLoginTx(s.sealer, LoginTransaction{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		ExpiresAt:    time.Now().Add(loginTxTTL),
	})
	if err != nil {
		return LoginRedirect{}, err
	}

	return LoginRedirect{
		AuthURL: s.google.AuthCodeURL(state, nonce, codeVerifier),
		OAuthTx: oauthTx,
	}, nil
}

// CompleteLogin refuses a forged or expired oauth_tx cookie, or a mismatched
// state, before Google is contacted. Replay is stopped by Google's codes being
// one-time, not by us.
func (s *service) CompleteLogin(ctx context.Context, in CallbackInput) (LoginResult, error) {
	if in.OAuthTx == "" {
		return LoginResult{}, fmt.Errorf("%w: no oauth_tx cookie", ErrLoginRejected)
	}

	tx, err := openLoginTx(s.sealer, in.OAuthTx)
	if err != nil {
		return LoginResult{}, err
	}

	if subtle.ConstantTimeCompare([]byte(in.State), []byte(tx.State)) != 1 {
		return LoginResult{}, fmt.Errorf("%w: state mismatch", ErrLoginRejected)
	}

	claims, err := s.google.Identify(ctx, in.Code, tx.CodeVerifier, tx.Nonce)
	if err != nil {
		return LoginResult{}, err
	}

	user, isNewUser, err := s.repo.UpsertGoogleIdentity(ctx, GoogleIdentityParams{
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		AvatarURL:     claims.Picture,
		HD:            claims.HD,
	})
	if err != nil {
		return LoginResult{}, err
	}

	// A fresh id every login defeats session fixation, where an attacker plants
	// a known session id before sign-in.
	sessionID, err := randomToken()
	if err != nil {
		return LoginResult{}, err
	}

	if err := s.repo.CreateSession(ctx, Session{ID: sessionID, UserID: user.ID}); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{SessionID: sessionID, IsNewUser: isNewUser, User: user}, nil
}

func (s *service) SessionUser(ctx context.Context, sessionID string) (*User, error) {
	now := time.Now()

	session, err := s.repo.GetSession(ctx, sessionID, now.Add(-s.ttl), now.Add(-s.idle))
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	// Extend the idle window on every read: the write is cheap, and skipping it
	// would let an active session expire mid-use.
	if err := s.repo.UpdateSessionLastSeenAt(ctx, sessionID); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *service) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, sessionID)
}

// randomToken mints a session id, state or nonce: 32 CSPRNG bytes, base64url so
// it is safe in a cookie and a URL.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
