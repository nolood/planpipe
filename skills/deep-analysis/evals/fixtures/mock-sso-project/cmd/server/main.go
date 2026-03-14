package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/acme/platform/internal/auth"
	"github.com/acme/platform/internal/config"
	"github.com/acme/platform/internal/tenant"
	"github.com/acme/platform/internal/user"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	db, err := config.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Repositories
	tenantRepo := tenant.NewRepository(db)
	userRepo := user.NewRepository(db)

	// Services
	keycloakClient := auth.NewKeycloakClient(cfg.Keycloak)
	tenantSvc := tenant.NewService(tenantRepo)
	userSvc := user.NewService(userRepo, tenantSvc)
	authSvc := auth.NewService(keycloakClient, userSvc, tenantSvc)

	// Middleware
	authMiddleware := auth.NewMiddleware(keycloakClient, tenantSvc)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/api/auth/login", auth.NewHandler(authSvc).Login)
		r.Post("/api/auth/logout", auth.NewHandler(authSvc).Logout)
		r.Post("/api/auth/refresh", auth.NewHandler(authSvc).RefreshToken)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		r.Get("/api/users/me", user.NewHandler(userSvc).GetCurrentUser)
		r.Get("/api/tenants/{tenantID}", tenant.NewHandler(tenantSvc).GetTenant)

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireRole("admin"))
			r.Get("/api/admin/users", user.NewHandler(userSvc).ListUsers)
			r.Put("/api/admin/tenants/{tenantID}", tenant.NewHandler(tenantSvc).UpdateTenant)
		})
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")
	srv.Shutdown(context.Background())
}
