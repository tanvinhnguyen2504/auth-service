package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
)

const googleIssuer = "https://accounts.google.com"

// ErrLoginRejected covers every unacceptable Google round trip — bad state,
// failed exchange, bad ID token, unverified email. The client sees only that.
var ErrLoginRejected = errors.New("auth: login rejected")

// ErrDomainNotAllowed is separate from ErrLoginRejected because it maps to 403,
// not 401: the identity is proven, it is simply not permitted here.
var ErrDomainNotAllowed = errors.New("auth: google workspace domain not allowed")

// GoogleClaims is the subset of verified ID token claims this app trusts.
type GoogleClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Nonce         string `json:"nonce"`
	HD            string `json:"hd"`
}

type GoogleAuth struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

type IdentityProvider interface {
	AuthCodeURL(state, nonce, codeVerifier string) string
	Identify(ctx context.Context, code, codeVerifier, expectedNonce string) (*GoogleClaims, error)
}

func NewGoogleAuth(ctx context.Context, cfg config.Config) (*GoogleAuth, error) {
	provider, err := oidc.NewProvider(ctx, googleIssuer)
	if err != nil {
		return nil, fmt.Errorf("auth: google oidc discovery: %w", err)
	}

	return &GoogleAuth{
		oauth: &oauth2.Config{
			ClientID:     cfg.Auth.GoogleOAuthClientID,
			ClientSecret: cfg.Auth.GoogleOAuthClientSecret,
			RedirectURL:  cfg.Auth.GoogleOAuthRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     provider.Endpoint(),
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.Auth.GoogleOAuthClientID}),
	}, nil
}

func (g *GoogleAuth) AuthCodeURL(state, nonce, codeVerifier string) string {
	return g.oauth.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(codeVerifier),
	)
}

func (g *GoogleAuth) Identify(ctx context.Context, code, codeVerifier, expectedNonce string) (*GoogleClaims, error) {
	token, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("%w: exchange code: %w", ErrLoginRejected, err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("%w: token response carried no id_token", ErrLoginRejected)
	}

	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("%w: verify id_token: %w", ErrLoginRejected, err)
	}

	var c GoogleClaims
	if err := idToken.Claims(&c); err != nil {
		return nil, fmt.Errorf("%w: decode claims: %w", ErrLoginRejected, err)
	}

	// go-oidc verifies signature, iss, aud and exp; the rest is ours. Two empty
	// strings compare equal, so a missing nonce must be rejected before that.
	if expectedNonce == "" || c.Nonce == "" {
		return nil, fmt.Errorf("%w: id_token carried no nonce", ErrLoginRejected)
	}
	if subtle.ConstantTimeCompare([]byte(c.Nonce), []byte(expectedNonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", ErrLoginRejected)
	}

	if c.Subject == "" {
		return nil, fmt.Errorf("%w: id_token carried no subject", ErrLoginRejected)
	}

	// Nothing is keyed on email, but it is displayed, so an unverified address is
	// refused rather than stored.
	if !c.EmailVerified {
		return nil, fmt.Errorf("%w: email not verified", ErrLoginRejected)
	}

	return &c, nil
}
