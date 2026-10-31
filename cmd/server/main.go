package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/nguyentanvinh/auth-be/backend/docs/swagger"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/config"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/db"
	"github.com/nguyentanvinh/auth-be/backend/internal/platform/router"
	"github.com/rs/cors"
)

// @title                      Auth Service
// @version                    1.0
// @description                Google sign-in, server-side sessions.
// @BasePath                   /api/v1
//
// host is deliberately unset so Swagger UI calls whichever origin served the
// page — localhost and the ngrok tunnel both work without regenerating.
//
// @securityDefinitions.apikey CSRFToken
// @in                         header
// @name                       X-CSRF-Token
// @description                Echo the value of the `csrf` cookie. Required on every state-changing /api/v1 call.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	log.Printf("database connected (%s)", cfg.RedactedDatabaseTarget())

	routes, err := (&router.Router{Config: cfg, Pool: pool}).NewRouter(ctx)
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	// Exactly one allowed origin, with credentials: the session lives in a
	// cookie, which the browser sends cross-origin only when both are set.
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedHeaders:   config.AllowHeadersAPI,
		AllowCredentials: true,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead,
		},
	})

	// TODO vincent: take these from cfg.Server once those fields are wired.
	server := &http.Server{
		Addr:              cfg.ServerPort(),
		Handler:           corsHandler.Handler(routes),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.ServerPort())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
