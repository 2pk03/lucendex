package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/lucendex/backend/internal/api"
	"github.com/lucendex/backend/internal/kv"
	"github.com/lucendex/backend/internal/router"
	"github.com/lucendex/backend/internal/store"
)

var (
	version     = "dev"
	buildTime   = "unknown"
	showVersion = flag.Bool("version", false, "Show version and exit")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("lucendex-api, build %s\n", buildTime)
		return
	}

	ctx := context.Background()

	// Use DATABASE_URL if provided (for Docker), otherwise fall back to individual vars
	apiConnStr := getEnv("DATABASE_URL", "")
	if apiConnStr == "" {
		dbHost := getEnv("DB_HOST", "localhost")
		dbPort := getEnv("DB_PORT", "5432")
		dbName := getEnv("DB_NAME", "lucendex")
		dbUser := getEnv("DB_USER", "api_ro")
		dbPassword := getEnv("DB_PASSWORD", "")
		dbSSLMode := getEnv("DB_SSLMODE", "require")
		apiConnStr = fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
			dbHost, dbPort, dbName, dbUser, dbPassword, dbSSLMode)
	}

	db, err := sql.Open("postgres", apiConnStr)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	internalToken := getEnv("INTERNAL_TOKEN", "")

	// Router uses same database for read-only access (reuse connection)
	routerStore, err := store.NewRouterStore(apiConnStr)
	if err != nil {
		log.Fatalf("failed to create router store: %v", err)
	}
	defer routerStore.Close()

	validator := router.NewValidator()
	pathfinder := router.NewPathfinder([]router.AMMPool{}, []router.Offer{})
	breaker := router.NewCircuitBreaker(0.05)
	quoteEngine := router.NewQuoteEngine(validator, pathfinder, breaker, kvStore, 20)
	r := router.NewRouter(quoteEngine, routerStore, kvStore)

	apiStore := api.NewPostgresStore(db)

	authMiddleware := api.NewAuthMiddleware(apiStore)
	rateLimiter := api.NewRateLimiter(kvStore, apiStore)
	handlers := api.NewHandlers(r, apiStore, kvStore, internalToken)

	mux := http.NewServeMux()

	partnerMux := http.NewServeMux()
	partnerMux.HandleFunc("/partner/v1/quote", handlers.QuoteHandler)
	partnerMux.HandleFunc("/partner/v1/pairs", handlers.PairsHandler)
	partnerMux.HandleFunc("/partner/v1/usage", handlers.UsageHandler)
	partnerMux.HandleFunc("/partner/v1/health", handlers.HealthHandler)

	mux.Handle("/partner/", rateLimiter.Middleware(authMiddleware.Middleware(partnerMux)))
	mux.HandleFunc("/internal/v1/ledger", handlers.LedgerUpdateHandler)

	port := getEnv("API_PORT", "8080")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("API server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
