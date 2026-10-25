package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/handler"
)

// CSRFHeader is the header the SPA echoes the csrf cookie back in. It is also a
// CORS AllowedHeader, so a cross-origin page cannot set it without a preflight.
const CSRFHeader = "X-CSRF-Token"

// CSRF enforces double-submit: the browser attaches the session cookie even for
// a foreign page, so holding it proves nothing — reading it does.
//
// cookieName is injected, not imported from auth: this package sits below the
// feature packages.
type CSRF struct {
	handler.Handler
	cookieName string
}

func NewCSRF(cfg *config.Config, cookieName string) *CSRF {
	return &CSRF{Handler: handler.Handler{Config: cfg}, cookieName: cookieName}
}

// isSafeMethod reports whether the method is read-only. Such requests carry no
// token, which is why no path exemption list is needed: the Google login routes
// are all GET.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

func (c *CSRF) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(c.cookieName)
		if err != nil || cookie.Value == "" {
			c.ReturnWithError(w, r, handler.Error(handler.ECForbidden, errors.New("csrf: no csrf cookie")))
			return
		}

		// Constant time: the value is one an attacker is guessing at, so the
		// comparison must not leak how long a shared prefix was.
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(CSRFHeader)), []byte(cookie.Value)) != 1 {
			c.ReturnWithError(w, r, handler.Error(handler.ECForbidden, errors.New("csrf: token does not match cookie")))
			return
		}

		next.ServeHTTP(w, r)
	})
}
