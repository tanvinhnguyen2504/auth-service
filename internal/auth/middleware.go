package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/handler"
)

type userKey struct{}

func UserFrom(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userKey{}).(*User)
	return user, ok
}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := h.readCookie(r, sessionCookie)
		if sessionID == "" {
			h.ReturnWithError(w, r, handler.Error(handler.ECUnauthenticated, errors.New("no session cookie")))
			return
		}

		user, err := h.service.SessionUser(r.Context(), sessionID)
		if err != nil {
			h.clearCookie(w, sessionCookie, "/")
			h.ReturnWithError(w, r, handler.Error(handler.ECUnauthenticated, err))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}
