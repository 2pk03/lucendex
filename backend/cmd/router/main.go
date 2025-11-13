package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucendex/backend/internal/kv"
	"github.com/lucendex/backend/internal/router"
	"github.com/lucendex/backend/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}

	routerBps := 20
	threshold := 0.05

	kvStore := kv.NewMemoryStore()
	defer kvStore.Close()

	dbStore, err := store.NewRouterStore(dbURL)
	if err != nil {
		log.Fatalf("NewRouterStore failed: %v", err)
	}
	defer dbStore.Close()

	validator := router.NewValidator()
	breaker := router.NewCircuitBreaker(threshold)

	pools := []router.AMMPool{}
	offers := []router.Offer{}
	pathfinder := router.NewPathfinder(pools, offers)

	quoteEngine := router.NewQuoteEngine(validator, pathfinder, breaker, kvStore, routerBps)

	log.Printf("Router started: routerBps=%d, threshold=%.2f%%", routerBps, threshold*100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startCleanupLoop(ctx, dbStore)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Println("Shutting down...")

	_ = quoteEngine
}

func startCleanupLoop(ctx context.Context, store *store.RouterStore) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			
			count1, _ := store.CleanupExpiredRateLimits(cleanupCtx)
			count2, _ := store.CleanupUsedQuotes(cleanupCtx)
			
			if count1 > 0 || count2 > 0 {
				fmt.Printf("Cleaned: %d rate limits, %d quotes\n", count1, count2)
			}
			
			cancel()
		}
	}
}
