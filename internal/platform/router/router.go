package router

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nguyentanvinh/auth-be/backend/internal/auth"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

type Router struct {
	Config *config.Config
	Pool   *pgxpool.Pool
}

const PrefixAPIV1 = "/api/v1"

func (r *Router) NewRouter(ctx context.Context) (*mux.Router, error) {
	router := mux.NewRouter()
	router.Use(middleware.SecurityHeaders)

	if r.Config.IsProduction() {
		router.Use(middleware.HSTS)
	}

	router.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	}).Methods(http.MethodGet)

	router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("doc.json"),
	)).Methods(http.MethodGet)

	api := router.PathPrefix(PrefixAPIV1).Subrouter()

	api.Use(middleware.NewAppID(r.Config, PrefixAPIV1).Require)
	api.Use(middleware.NewCSRF(r.Config, auth.CSRFCookieName(r.Config)).Require)

	r.registerHealthRoute(api)
	if err := r.registerAuthRoute(ctx, api); err != nil {
		return nil, err
	}

	return router, nil
}
