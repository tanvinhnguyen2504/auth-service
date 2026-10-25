package auth

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/handler"
)

type Handler struct {
	handler.Handler
	service Service
}

func NewHandler(cfg *config.Config, service Service) Handler {
	return Handler{
		Handler: handler.Handler{Config: cfg},
		service: service,
	}
}

// loginErrorReason is the ?error= value on the redirect back to the SPA. A
// separate set from handler.ErrorCode — failed navigation, not a failed JSON
// call — and read by different frontend code.
type loginErrorReason string

const (
	reasonCancelled   loginErrorReason = "cancelled"    // user pressed Cancel at Google
	reasonLoginFailed loginErrorReason = "login_failed" // bad state, bad id_token, expired tx
	reasonForbidden   loginErrorReason = "forbidden"    // hd not allowed
	reasonServerError loginErrorReason = "server_error" // our fault
)

// Login starts the Google redirect flow.
//
//	@Summary     Start Google sign-in
//	@Description Sets the sealed oauth_tx cookie and 302s to Google. Open this in the browser, not from Swagger UI's "Try it out" — it is a navigation, not an XHR.
//	@Tags        auth
//	@Success     302 "redirect to Google"
//	@Failure     302 "redirect to FRONTEND_URL/login?error=server_error"
//	@Router      /auth/google/login [get]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	login, err := h.service.StartGoogleLogin(r.Context())
	if err != nil {
		h.failLogin(w, r, reasonServerError, err)
		return
	}

	h.setOAuthTxCookie(w, login.OAuthTx)
	h.noStore(w)
	http.Redirect(w, r, login.AuthURL, http.StatusFound)
}

// CallbackLogin is where Google sends the user back.
//
//	@Summary     Google OAuth callback
//	@Description Verifies state and the ID token, upserts the user, issues the sid + csrf cookies, then 302s to the frontend. Called by Google, never by the SPA.
//	@Tags        auth
//	@Param       code  query string true  "authorization code from Google"
//	@Param       state query string true  "opaque state, must match the oauth_tx cookie"
//	@Param       error query string false "set by Google when the user cancels"
//	@Success     302 "redirect to FRONTEND_URL"
//	@Failure     302 "redirect to FRONTEND_URL/login?error=cancelled|login_failed|forbidden|server_error"
//	@Router      /auth/google/callback [get]
func (h *Handler) CallbackLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	oauthTx := h.readCookie(r, oauthTxCookie)
	h.clearCookie(w, oauthTxCookie, OAuthTxCookiePath)

	if reason := q.Get("error"); reason != "" {
		h.failLogin(w, r, reasonCancelled, errors.New("google: "+reason))
		return
	}

	if q.Get("code") == "" || q.Get("state") == "" {
		h.failLogin(w, r, reasonLoginFailed, errors.New("callback missing code or state"))
		return
	}

	login, err := h.service.CompleteLogin(r.Context(), CallbackInput{
		Code:    q.Get("code"),
		State:   q.Get("state"),
		OAuthTx: oauthTx,
	})
	switch {
	case errors.Is(err, ErrDomainNotAllowed):
		h.failLogin(w, r, reasonForbidden, err)
		return
	case errors.Is(err, ErrLoginRejected):
		h.failLogin(w, r, reasonLoginFailed, err)
		return
	case err != nil:
		h.failLogin(w, r, reasonServerError, err)
		return
	}

	csrfToken, err := randomToken()
	if err != nil {
		h.failLogin(w, r, reasonServerError, err)
		return
	}

	h.setSessionCookie(w, login.SessionID)
	h.setCSRFCookie(w, csrfToken)
	h.noStore(w)

	target := h.Config.FrontendURL
	if login.IsNewUser {
		target += "/onboarding"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// Me returns the signed-in user.
//
//	@Summary     Current user
//	@Tags        auth
//	@Produce     json
//	@Success     200 {object} config.SuccessResponse{data=auth.MeResponse}
//	@Failure     401 {object} handler.ErrorResponse "UNAUTHENTICATED"
//	@Router      /me [get]
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		h.ReturnWithError(w, r, handler.Error(handler.ECUnauthenticated, errors.New("no user in context")))
		return
	}

	h.noStore(w)
	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: MeResponse{User: newUserResponse(user)}})
}

// Logout revokes this session.
//
//	@Summary     Sign out
//	@Description Deletes the session row and clears the sid + csrf cookies. Other devices stay signed in.
//	@Tags        auth
//	@Produce     json
//	@Success     204 "signed out"
//	@Failure     403 {object} handler.ErrorResponse "FORBIDDEN — csrf header missing or mismatched"
//	@Failure     500 {object} handler.ErrorResponse "INTERNAL"
//	@Security    CSRFToken
//	@Router      /auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context(), h.readCookie(r, sessionCookie)); err != nil {
		h.ReturnWithError(w, r, handler.Error(handler.ECInternal, err))
		return
	}

	h.clearCookie(w, sessionCookie, "/")
	h.clearCookie(w, csrfCookie, "/")
	h.noStore(w)
	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) failLogin(w http.ResponseWriter, r *http.Request, reason loginErrorReason, cause error) {
	log.Printf("auth: login failed (%s): %v", reason, cause)

	h.noStore(w)
	target := h.Config.FrontendURL + "/login?" + url.Values{"error": {string(reason)}}.Encode()
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *Handler) noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
