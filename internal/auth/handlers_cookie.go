package auth

import (
	"net/http"
	"time"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
)

const (
	sessionCookie = "sid"      // server-side session id
	oauthTxCookie = "oauth_tx" // binds a login transaction to this browser
	csrfCookie    = "csrf"     // double-submit token; readable by JS on purpose
)

// OAuthTxCookiePath scopes the oauth_tx cookie so nothing outside the Google
// login routes can read it. It MUST stay a prefix of the mounted callback path:
// the browser withholds the cookie on a path mismatch, and the callback then
// fails with "no oauth_tx cookie" — a silent, total login outage with no
// compile error. Asserted against the router in TestOAuthTxCookiePathCoversRoutes.
const OAuthTxCookiePath = "/api/v1/auth/google"

// secure keeps the Secure attribute and the __Host- prefix it requires in one
// place, so the two cannot disagree.
func (h *Handler) secure() bool {
	return h.Config.IsProduction()
}

// cookieName adds the __Host- prefix in production, so no subdomain can
// overwrite the cookie.
func (h *Handler) cookieName(base string) string {
	return cookieNameFor(h.Config, base)
}

func cookieNameFor(cfg *config.Config, base string) string {
	if cfg.IsProduction() {
		return "__Host-" + base
	}
	return base
}

// setCookie is the only place cookie attributes are chosen. ttl becomes Max-Age,
// not Expires, so a skewed client clock cannot mis-expire the cookie. SameSite is
// Lax, not Strict: Google's redirect back is a cross-site top-level navigation.
func (h *Handler) setCookie(w http.ResponseWriter, name, value, path string, ttl time.Duration, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName(name),
		Value:    value,
		Path:     path,
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: httpOnly,
		Secure:   h.secure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name, path string) {
	h.setCookie(w, name, "", path, -time.Second, true) // MaxAge < 0 deletes it
}

func (h *Handler) readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(h.cookieName(name))
	if err != nil {
		return ""
	}
	return c.Value
}

// setSessionCookie keeps the id HttpOnly: no script reads it, so XSS cannot
// steal it the way it could from localStorage.
func (h *Handler) setSessionCookie(w http.ResponseWriter, sessionID string) {
	h.setCookie(w, sessionCookie, sessionID, "/", h.Config.Auth.SessionTTL, true)
}

// setCSRFCookie is intentionally not HttpOnly: the SPA must read it to echo it
// back in a header. It is not a credential, only proof of same-origin access.
func (h *Handler) setCSRFCookie(w http.ResponseWriter, token string) {
	h.setCookie(w, csrfCookie, token, "/", h.Config.Auth.SessionTTL, false)
}

// setOAuthTxCookie carries the sealed login transaction, scoped to the callback
// path and expiring with the transaction so nothing else can reach it.
func (h *Handler) setOAuthTxCookie(w http.ResponseWriter, sealed string) {
	h.setCookie(w, oauthTxCookie, sealed, OAuthTxCookiePath, loginTxTTL, true)
}

// CSRFCookieName is the resolved name the CSRF middleware must read. It takes
// config because the router wires middleware before any Handler exists.
func CSRFCookieName(cfg *config.Config) string {
	return cookieNameFor(cfg, csrfCookie)
}
