package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders sets the response headers that cost nothing and close off
// whole attack classes. Deliberately not configurable: a header that can be
// switched off per environment eventually is, where it was needed most.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// A JSON body sniffed as HTML is an XSS vector on the origin whose
		// cookies carry the session.
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")

		// no-referrer: the callback URL carries Google's authorization code and
		// must never travel to another site.
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")

		// Swagger UI is real HTML that loads its own scripts, so it is the one
		// exemption from the locked-down policy.
		if !strings.HasPrefix(r.URL.Path, "/swagger/") {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}

		next.ServeHTTP(w, r)
	})
}

// HSTS is separate from SecurityHeaders because it cannot be taken back for
// max-age: sent from a host ever reached over https on a shared hostname, it
// breaks that host. Wire it only in production.
func HSTS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}
