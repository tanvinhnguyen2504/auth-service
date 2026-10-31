package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/handler"
)

const (
	AppIDHeader = "app-id"
	AppIDParam  = "app-id"
)

func ExtractAppIDFromContext(ctx context.Context) (string, bool) {
	appID, ok := ctx.Value(config.ContextAppID).(string)
	return appID, ok
}

type AppID struct {
	handler.Handler
	allowed map[string]struct{}
	// pathPrefix is the mount point of the subrouter this middleware runs on,
	// injected rather than imported from router: that package imports this one.
	pathPrefix string
}

func NewAppID(cfg *config.Config, pathPrefix string) *AppID {
	allowed := make(map[string]struct{}, len(cfg.Apps.AllowedIDs))
	for _, id := range cfg.Apps.AllowedIDs {
		allowed[id] = struct{}{}
	}

	return &AppID{
		Handler: handler.Handler{Config: cfg},
		allowed: allowed,
		// A trailing slash would leave it in the trimmed path and match nothing,
		// so "/api/v1" and "/api/v1/" are accepted as the same prefix.
		pathPrefix: strings.TrimSuffix(pathPrefix, "/"),
	}
}

// appIDExcludePaths are the routes whose caller is not one of our frontends and
// so cannot be asked for an app id. Paths are relative to AppID.pathPrefix, so
// the api version lives in exactly one place: router.PrefixAPIV1.
var appIDExcludePaths = map[string]struct{}{
	"/":                     {},
	"/health":               {},
	"/auth/google/callback": {}, // Google redirects the browser here; it knows nothing of our app ids.
}

func (a *AppID) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, exclude := appIDExcludePaths[strings.TrimPrefix(r.URL.Path, a.pathPrefix)]; exclude {
			next.ServeHTTP(w, r)
			return
		}

		appID := r.Header.Get(AppIDHeader)
		if appID == "" {
			appID = r.URL.Query().Get(AppIDParam)
		}

		if appID == "" {
			a.ReturnWithError(w, r, handler.ErrorWithMessage(
				handler.ECValidationError,
				"Missing app-id.",
				errors.New("app-id: absent from both header and query"),
			))
			return
		}

		if _, ok := a.allowed[appID]; !ok {
			a.ReturnWithError(w, r, handler.Error(handler.ECForbidden,
				fmt.Errorf("app-id: %q is not an allowed app", appID)))
			return
		}

		ctxWithAppID := context.WithValue(r.Context(), config.ContextAppID, appID)
		next.ServeHTTP(w, r.WithContext(ctxWithAppID))
	})
}
