package main

import (
	"log"
	"net/http"

	"github.com/example/multi-tenant-app/config"
	"github.com/example/multi-tenant-app/internal/auth"
	"github.com/example/multi-tenant-app/internal/health"
	"github.com/example/multi-tenant-app/internal/middleware"
	"github.com/example/multi-tenant-app/internal/tenant"
	"github.com/example/multi-tenant-app/internal/user"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := config.Load()

	db := mustConnectDB(cfg.DatabaseURL)
	defer db.Close()

	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo)

	tenantRepo := tenant.NewRepository(db)

	authHandler := auth.NewHandler(userService, cfg.JWTSecret)

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestLogger)

	// Health check
	r.Get("/health", health.HandleHealth)

	// Auth routes
	r.Post("/auth/login", authHandler.HandleLogin)
	r.Post("/auth/register", authHandler.HandleRegister)

	// Tenant routes
	r.Get("/tenants/{id}", tenant.NewHandler(tenantRepo).HandleGet)

	log.Printf("Starting server on %s", cfg.ServerAddr)
	log.Fatal(http.ListenAndServe(cfg.ServerAddr, r))
}

func mustConnectDB(url string) *pgxPool {
	// simplified for fixture
	return nil
}

type pgxPool struct{}

func (p *pgxPool) Close() {}
