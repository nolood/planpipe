package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/acme/analytics/internal/analytics"
	"github.com/acme/analytics/internal/clickhouse"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	chURL := os.Getenv("CLICKHOUSE_URL")
	if chURL == "" {
		chURL = "clickhouse://localhost:9000/analytics"
	}

	chClient, err := clickhouse.NewClient(chURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to ClickHouse")
	}
	defer chClient.Close()

	analyticsSvc := analytics.NewService(chClient)
	resolver := analytics.NewResolver(analyticsSvc)

	gqlHandler := handler.NewDefaultServer(analytics.NewExecutableSchema(analytics.Config{
		Resolvers: resolver,
	}))

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS for dashboard frontend
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Handle("/graphql", gqlHandler)
	r.Handle("/playground", playground.Handler("Analytics", "/graphql"))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	srv := &http.Server{Addr: ":" + port, Handler: r}

	go func() {
		log.Info().Str("port", port).Msg("analytics API starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	srv.Shutdown(context.Background())
}
