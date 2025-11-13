package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucendex/backend/internal/parser"
	"github.com/lucendex/backend/internal/store"
	"github.com/lucendex/backend/internal/xrpl"
)

var (
	// Set at build time via -ldflags
	version = "dev"
	buildTime = "unknown"
)

var (
	rippledWS   = flag.String("rippled-ws", getEnv("RIPPLED_WS", "ws://localhost:6006"), "rippled Full-History WebSocket URL")
	rippledHTTP = flag.String("rippled-http", getEnv("RIPPLED_HTTP", "http://localhost:51237"), "rippled HTTP RPC URL for gap detection")
	dbConnStr   = flag.String("db", getEnv("DATABASE_URL", ""), "PostgreSQL connection string")
	verbose     = flag.Bool("v", getEnv("VERBOSE", "") == "true", "Enable verbose logging")
	showVersion = flag.Bool("version", false, "Show version and exit")
	startLedger = flag.Uint64("start-ledger", 99984580, "Earliest ledger to index (Nov 1, 2025 00:00 UTC ≈ ledger 99984580)")
)

// getEnv retrieves environment variable or returns default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// logVerbose logs only when verbose mode is enabled
func logVerbose(format string, v ...interface{}) {
	if *verbose {
		log.Printf(format, v...)
	}
}

// logInfo logs to stdout (normal operation)
func logInfo(format string, v ...interface{}) {
	log.SetOutput(os.Stdout)
	log.Printf(format, v...)
	log.SetOutput(os.Stderr) // Reset to stderr for errors
}

// logError logs to stderr (errors only)
func logError(format string, v ...interface{}) {
	log.SetOutput(os.Stderr)
	log.Printf(format, v...)
}

func main() {
	flag.Parse()
	
	// Set log output to stdout (stderr only for Fatal errors)
	log.SetOutput(os.Stdout)
	
	if *showVersion {
		log.Printf("lucendex-indexer, build %s", buildTime)
		return
	}
	
	log.Printf("lucendex-indexer, build %s", buildTime)
	log.Printf("Rippled WS: %s", *rippledWS)
	
	// Validate required configuration
	if *dbConnStr == "" {
		log.Fatal("DATABASE_URL environment variable or -db flag is required")
	}
	
	// Connect to database with retry logic
	log.Printf("Connecting to database...")
	var db *store.Store
	var dbConnectStart time.Time
	for retry := 0; retry < 10; retry++ {
		dbConnectStart = time.Now()
		var err error
		db, err = store.NewStore(*dbConnStr)
		if err == nil {
			// Log successful connection
			if db != nil {
				db.LogConnectionEvent("postgres", "success", retry+1, nil, int(time.Since(dbConnectStart).Milliseconds()), nil)
			}
			break
		}
		// Log failed attempt
		log.Printf("Database connection attempt %d/10 failed: %v", retry+1, err)
		if db != nil {
			db.LogConnectionEvent("postgres", "failure", retry+1, err, int(time.Since(dbConnectStart).Milliseconds()), nil)
		}
		if retry < 9 {
			db.LogConnectionEvent("postgres", "retry", retry+2, nil, 0, map[string]interface{}{"retry_delay_seconds": retry + 1})
		}
		time.Sleep(time.Second * time.Duration(retry+1))
	}
	if db == nil {
		log.Fatal("Failed to connect to database after 10 retries")
	}
	defer db.Close()
	log.Printf("✓ Database connected")
	
	// Check for last checkpoint
	ctx := context.Background()
	checkpoint, err := db.GetLastCheckpoint(ctx)
	if err != nil {
		log.Fatalf("Failed to get last checkpoint: %v", err)
	}
	
	// Get current ledger via HTTP RPC (before WebSocket to avoid conflict)
	httpStart := time.Now()
	db.LogConnectionEvent("rippled-http", "attempt", 1, nil, 0, map[string]interface{}{"url": *rippledHTTP})
	serverInfo, err := xrpl.GetServerInfoHTTP(*rippledHTTP)
	if err != nil {
		db.LogConnectionEvent("rippled-http", "failure", 1, err, int(time.Since(httpStart).Milliseconds()), map[string]interface{}{"url": *rippledHTTP})
		log.Printf("Warning: Failed to get server info via HTTP: %v", err)
		log.Printf("Continuing without gap detection...")
	} else {
		db.LogConnectionEvent("rippled-http", "success", 1, nil, int(time.Since(httpStart).Milliseconds()), map[string]interface{}{"url": *rippledHTTP})
	}
	
	var currentLedger uint64
	if serverInfo != nil {
		currentLedger = serverInfo.Result.Info.ValidatedLedger.Seq
		log.Printf("Current validated ledger: %d", currentLedger)
	}
	
	// Connect to rippled WebSocket
	log.Printf("Connecting to rippled...")
	client := xrpl.NewClient(*rippledWS)
	
	wsStart := time.Now()
	db.LogConnectionEvent("rippled-ws", "attempt", 1, nil, 0, map[string]interface{}{"url": *rippledWS})
	if err := client.Connect(); err != nil {
		db.LogConnectionEvent("rippled-ws", "failure", 1, err, int(time.Since(wsStart).Milliseconds()), map[string]interface{}{"url": *rippledWS})
		log.Fatalf("Failed to connect to rippled: %v", err)
	}
	db.LogConnectionEvent("rippled-ws", "success", 1, nil, int(time.Since(wsStart).Milliseconds()), map[string]interface{}{"url": *rippledWS})
	defer client.Close()
	log.Printf("✓ Connected to rippled")
	
	// Subscribe to ledger stream
	if err := client.Subscribe(); err != nil {
		log.Fatalf("Failed to subscribe to ledger stream: %v", err)
	}
	log.Printf("✓ Subscribed to ledger stream")
	
	// Detect gap and backfill if needed
	if checkpoint != nil && currentLedger > 0 {
		gap := currentLedger - uint64(checkpoint.LedgerIndex)
		if gap > 1 {
			const historyRetention = 2048 // History node retention
			missingCount := gap - 1
			
			if missingCount <= historyRetention {
				log.Printf("⚠ Gap detected: %d ledgers (within history)", missingCount)
				log.Printf("Starting background backfill...")
				
				// Backfill in background
				go func() {
					backfillClient := xrpl.NewClientWithBuffer(*rippledWS, 10000)
					if err := backfillClient.Connect(); err != nil {
						log.Printf("Failed to connect backfill client: %v", err)
						return
					}
					defer backfillClient.Close()
					
					for i := uint64(checkpoint.LedgerIndex + 1); i < currentLedger; i++ {
						ledger, err := backfillClient.FetchLedgerSync(i)
						if err != nil {
							log.Printf("Failed to backfill ledger %d: %v", i, err)
							continue
						}
						
						if err := processLedger(ctx, db, ledger, parser.NewAMMParser(), parser.NewOrderbookParser()); err != nil {
							log.Printf("Error processing backfill ledger %d: %v", i, err)
						}
						
						if i%100 == 0 {
							log.Printf("Backfill progress: %d/%d ledgers", i-uint64(checkpoint.LedgerIndex), missingCount)
						}
					}
					
					log.Printf("✓ Backfill complete: %d ledgers", missingCount)
				}()
			} else {
				log.Printf("⚠ Large gap detected: %d ledgers (> history retention)", missingCount)
				log.Printf("Skipping backfill - resuming from current ledger")
			}
		} else {
			log.Printf("✓ No gap detected - resuming from ledger %d", checkpoint.LedgerIndex+1)
		}
	} else if checkpoint != nil {
		log.Printf("Resuming from checkpoint at ledger %d", checkpoint.LedgerIndex)
	} else {
		log.Printf("No checkpoint found - starting fresh")
	}
	
	// Create parsers for live processing
	ammParser := parser.NewAMMParser()
	orderbookParser := parser.NewOrderbookParser()
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	log.Printf("✓ Indexer running - waiting for ledgers...")
	
	// Main processing loop
	for {
		select {
		case <-sigChan:
			log.Printf("Shutdown signal received - closing gracefully")
			return
			
		case err := <-client.ErrorChan():
			log.Printf("Error from rippled client: %v", err)
			
		case ledger := <-client.LedgerChan():
			if err := processLedger(ctx, db, ledger, ammParser, orderbookParser); err != nil {
				log.Printf("Error processing ledger %d: %v", ledger.LedgerIndex, err)
			}
		}
	}
}

// hasLucendexQuoteHash checks if a transaction has a Lucendex quote hash in memo
func hasLucendexQuoteHash(tx map[string]interface{}) ([]byte, bool) {
	memos, ok := tx["Memos"].([]interface{})
	if !ok || len(memos) == 0 {
		return nil, false
	}
	
	for _, memo := range memos {
		memoObj, ok := memo.(map[string]interface{})
		if !ok {
			continue
		}
		
		memoWrapper, ok := memoObj["Memo"].(map[string]interface{})
		if !ok {
			continue
		}
		
		// Look for MemoType = "lucendex/quote" (hex encoded)
		memoType, ok := memoWrapper["MemoType"].(string)
		if ok && memoType == "6c7563656e6465782f71756f7465" { // "lucendex/quote" in hex
			// Extract MemoData (should be 32-byte Blake2b hash)
			memoData, ok := memoWrapper["MemoData"].(string)
			if ok && len(memoData) == 64 { // 32 bytes = 64 hex chars
				// Convert hex to bytes
				quoteHash := make([]byte, 32)
				for i := 0; i < 32; i++ {
					fmt.Sscanf(memoData[i*2:i*2+2], "%02x", &quoteHash[i])
				}
				return quoteHash, true
			}
		}
	}
	
	return nil, false
}

// extractTradeDetails extracts trade details from a Payment transaction
func extractTradeDetails(tx map[string]interface{}) (inAsset, outAsset, amountIn, amountOut string, ok bool) {
	// Get Account (sender)
	account, accountOk := tx["Account"].(string)
	if !accountOk {
		return
	}
	
	// Get Amount (what was sent)
	amount, amountOk := tx["Amount"].(map[string]interface{})
	if !amountOk {
		// Simple XRP amount
		amountStr, amountStrOk := tx["Amount"].(string)
		if amountStrOk {
			inAsset = "XRP"
			amountIn = amountStr
		} else {
			return
		}
	} else {
		// IOU amount
		currency, _ := amount["currency"].(string)
		issuer, _ := amount["issuer"].(string)
		value, _ := amount["value"].(string)
		inAsset = fmt.Sprintf("%s.%s", currency, issuer)
		amountIn = value
	}
	
	// Get DeliveredAmount from metadata (what was received)
	meta, metaOk := tx["meta"].(map[string]interface{})
	if !metaOk {
		return
	}
	
	delivered := meta["delivered_amount"]
	if delivered == nil {
		// Try DeliverMax
		delivered = tx["DeliverMax"]
		if delivered == nil {
			return
		}
	}
	
	if deliveredStr, isString := delivered.(string); isString {
		// Simple XRP
		outAsset = "XRP"
		amountOut = deliveredStr
	} else if deliveredMap, isMap := delivered.(map[string]interface{}); isMap {
		// IOU
		currency, _ := deliveredMap["currency"].(string)
		issuer, _ := deliveredMap["issuer"].(string)
		value, _ := deliveredMap["value"].(string)
		outAsset = fmt.Sprintf("%s.%s", currency, issuer)
		amountOut = value
	} else {
		return
	}
	
	_ = account // Unused for now, but we have it
	ok = true
	return
}

// processLedger processes a single ledger
func processLedger(
	ctx context.Context,
	db *store.Store,
	ledger *xrpl.LedgerResponse,
	ammParser *parser.AMMParser,
	orderbookParser *parser.OrderbookParser,
) error {
	start := time.Now()
	
	// Check if ledger already processed (duplicate prevention)
	existingCheckpoint, err := db.GetCheckpoint(ctx, int64(ledger.LedgerIndex))
	if err == nil && existingCheckpoint != nil {
		logVerbose("Skipping already processed ledger %d", ledger.LedgerIndex)
		return nil
	}
	
	// Verify ledger hash continuity (detect forks/corruption)
	if ledger.LedgerIndex > 1 {
		prevCheckpoint, err := db.GetCheckpoint(ctx, int64(ledger.LedgerIndex-1))
		if err == nil && prevCheckpoint != nil {
			// Verify parent hash matches previous ledger hash
			// Note: XRPL ledger data doesn't always include parent_hash in our response
			// We verify sequential processing instead
			logVerbose("Verified sequential ledger: %d follows %d", ledger.LedgerIndex, prevCheckpoint.LedgerIndex)
		}
	}
	
	log.Printf("Processing ledger %d (hash: %s, txns: %d)", 
		ledger.LedgerIndex, ledger.LedgerHash, ledger.TxnCount)
	
	// Process each transaction
	for _, tx := range ledger.Transactions {
		logVerbose("Processing tx %s (type: %s)", tx.Hash, tx.TransactionType)
		
		// Convert transaction to map for parser
		txMap := make(map[string]interface{})
		txBytes, err := json.Marshal(tx)
		if err != nil {
			log.Printf("Failed to marshal transaction: %v", err)
			continue
		}
		
		if err := json.Unmarshal(txBytes, &txMap); err != nil {
			log.Printf("Failed to unmarshal transaction: %v", err)
			continue
		}
		
		// Try AMM parser
		pool, err := ammParser.ParseTransaction(txMap, ledger.LedgerIndex, ledger.LedgerHash)
		if err != nil {
			log.Printf("AMM parser error on tx %s: %v", tx.Hash, err)
		} else if pool != nil {
			if err := db.UpsertAMMPool(ctx, pool); err != nil {
				log.Printf("Failed to upsert AMM pool: %v", err)
			} else {
				log.Printf("  ✓ AMM pool updated: %s/%s", pool.Asset1, pool.Asset2)
			}
		} else {
			logVerbose("  Skipped (not AMM transaction)")
		}
		
		// Try orderbook parser
		offer, err := orderbookParser.ParseTransaction(txMap, ledger.LedgerIndex, ledger.LedgerHash)
		if err != nil {
			log.Printf("Orderbook parser error on tx %s: %v", tx.Hash, err)
		} else if offer != nil {
			if err := db.UpsertOffer(ctx, offer); err != nil {
				log.Printf("Failed to upsert offer: %v", err)
			} else {
				if offer.Status == "invalid_parse" {
					logVerbose("  ⚠ Invalid offer stored: %v", offer.Meta["error"])
				} else {
					log.Printf("  ✓ Offer created: %s/%s @ %s", offer.BaseAsset, offer.QuoteAsset, offer.Price)
				}
			}
		} else {
			logVerbose("  Skipped (not orderbook transaction)")
		}
		
		// Check for Lucendex-executed trade
		if quoteHash, hasQuote := hasLucendexQuoteHash(txMap); hasQuote {
			// Extract trade details
			inAsset, outAsset, amountIn, amountOut, valid := extractTradeDetails(txMap)
			if valid {
				trade := &store.CompletedTrade{
					QuoteHash:    quoteHash,
					TxHash:       tx.Hash,
					Account:      tx.Account,
					InAsset:      inAsset,
					OutAsset:     outAsset,
					AmountIn:     amountIn,
					AmountOut:    amountOut,
					Route:        map[string]interface{}{"type": "unknown"}, // Route extraction TBD
					RouterFeeBps: 20, // Default, should extract from tx
					LedgerIndex:  int64(ledger.LedgerIndex),
					LedgerHash:   ledger.LedgerHash,
				}
				
				if err := db.InsertCompletedTrade(ctx, trade); err != nil {
					log.Printf("Failed to insert completed trade: %v", err)
				} else {
					log.Printf("  ✓ Lucendex trade recorded: %s→%s (quote: %x...)", 
						inAsset, outAsset, quoteHash[:4])
				}
			}
		}
		
		// Check for OfferCancel
		if tx.TransactionType == "OfferCancel" {
			account, seq, err := orderbookParser.ParseOfferCancel(txMap)
			if err == nil {
				if err := db.CancelOffer(ctx, account, seq, int64(ledger.LedgerIndex)); err != nil {
					log.Printf("Failed to cancel offer: %v", err)
				} else {
					log.Printf("  ✓ Offer cancelled: account=%s seq=%d", account, seq)
				}
			} else {
				logVerbose("  OfferCancel parse error: %v", err)
			}
		}
	}
	
	// Save checkpoint
	duration := time.Since(start)
	checkpoint := &store.LedgerCheckpoint{
		LedgerIndex:          int64(ledger.LedgerIndex),
		LedgerHash:           ledger.LedgerHash,
		CloseTime:            int64(ledger.LedgerTime),
		CloseTimeHuman:       time.Unix(int64(ledger.LedgerTime)+946684800, 0), // Ripple epoch to Unix
		TransactionCount:     ledger.TxnCount,
		ProcessingDurationMs: int(duration.Milliseconds()),
	}
	
	if err := db.SaveCheckpoint(ctx, checkpoint); err != nil {
		return err
	}
	
	log.Printf("✓ Ledger %d indexed in %v", ledger.LedgerIndex, duration)
	return nil
}
