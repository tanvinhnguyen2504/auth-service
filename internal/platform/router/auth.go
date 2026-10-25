package router

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nguyentanvinh/auth-be/backend/internal/auth"
	"github.com/nguyentanvinh/auth-be/backend/internal/secret"
)

func (r *Router) registerAuthRoute(ctx context.Context, api *mux.Router) error {
	googleAuth, err := auth.NewGoogleAuth(ctx, *r.Config)
	if err != nil {
		return fmt.Errorf("router: google auth: %w", err)
	}

	sealer, err := secret.NewBox(r.Config.Auth.EncryptionKey, r.Config.Auth.EncryptionKeyVersion)
	if err != nil {
		return fmt.Errorf("router: encryption key: %w", err)
	}

	authRepo := auth.NewRepo(r.Pool)

	authService := auth.NewService(googleAuth, authRepo, sealer, r.Config.Auth.SessionTTL, r.Config.Auth.SessionIdleTTL)
	authHandler := auth.NewHandler(r.Config, authService)

	auth.StartSessionCleanup(ctx, authRepo, r.Config.Auth.SessionTTL, r.Config.Auth.SessionIdleTTL)

	authRouter := api.PathPrefix("/auth/google").Subrouter()
	authRouter.HandleFunc("/login", authHandler.Login).Methods(http.MethodGet)
	authRouter.HandleFunc("/callback", authHandler.CallbackLogin).Methods(http.MethodGet)

	api.Handle("/auth/logout", http.HandlerFunc(authHandler.Logout)).Methods(http.MethodPost)
	api.Handle("/me",
		authHandler.RequireAuth(http.HandlerFunc(authHandler.Me)),
	).Methods(http.MethodGet)

	return nil
}
