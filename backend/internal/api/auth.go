package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ErrMissingAuthHeaders = errors.New("missing authentication headers")
	ErrInvalidTimestamp   = errors.New("invalid or expired timestamp")
	ErrReplayAttack       = errors.New("duplicate request-id (replay attack)")
	ErrInvalidPartner     = errors.New("invalid partner")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrPartnerSuspended   = errors.New("partner account suspended")
)

type AuthMiddleware struct {
	db DB
}

type DB interface {
	GetPartnerByID(ctx context.Context, partnerID uuid.UUID) (*Partner, error)
	GetAPIKeyByPublicKey(ctx context.Context, publicKey string) (*APIKey, error)
	GetActiveAPIKey(ctx context.Context, partnerID uuid.UUID) (*APIKey, error)
	CheckRequestID(ctx context.Context, requestID uuid.UUID, partnerID uuid.UUID) (bool, error)
	StoreRequestID(ctx context.Context, requestID uuid.UUID, partnerID uuid.UUID, expiresAt time.Time) error
	GetPartnerUsage(ctx context.Context, partnerID uuid.UUID, month string) (*UsageResponse, error)
	StoreQuoteRegistry(ctx context.Context, registry *QuoteRegistry) error
	GetIndexerLag(ctx context.Context) (int, error)
	UpdateNetworkLedger(ctx context.Context, ledger uint32) error
}

func NewAuthMiddleware(db DB) *AuthMiddleware {
	return &AuthMiddleware{db: db}
}

func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract headers
		partnerIDStr := r.Header.Get("X-Partner-Id")
		requestIDStr := r.Header.Get("X-Request-Id")
		timestamp := r.Header.Get("X-Timestamp")
		signature := r.Header.Get("X-Signature")

		if partnerIDStr == "" || requestIDStr == "" || timestamp == "" || signature == "" {
			writeError(w, http.StatusUnauthorized, ErrMissingAuthHeaders.Error())
			return
		}

		// Parse UUIDs
		partnerID, err := uuid.Parse(partnerIDStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid partner-id format")
			return
		}

		requestID, err := uuid.Parse(requestIDStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid request-id format")
			return
		}

		// Validate timestamp (60 second drift max)
		ts, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid timestamp format")
			return
		}

		drift := time.Since(ts)
		if drift < 0 {
			drift = -drift
		}
		if drift > 60*time.Second {
			writeError(w, http.StatusUnauthorized, ErrInvalidTimestamp.Error())
			return
		}

		// Check for replay attack
		exists, err := am.db.CheckRequestID(ctx, requestID, partnerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "authentication error")
			return
		}
		if exists {
			writeError(w, http.StatusUnauthorized, ErrReplayAttack.Error())
			return
		}

		// Load partner
		partner, err := am.db.GetPartnerByID(ctx, partnerID)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusUnauthorized, ErrInvalidPartner.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "authentication error")
			return
		}

		// Check partner status
		if partner.Status != "active" {
			writeError(w, http.StatusForbidden, ErrPartnerSuspended.Error())
			return
		}

		// Enforce max body size and hash body
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		bodyHash := sha256.Sum256(body)

		// Build canonical request
		canonical := fmt.Sprintf("%s\n%s\n%s\n%x\n%s",
			r.Method,
			r.URL.Path,
			r.URL.RawQuery,
			bodyHash,
			timestamp,
		)

		// Get API key
		apiKey, err := am.db.GetActiveAPIKey(ctx, partnerID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, ErrInvalidPartner.Error())
			return
		}

		// Verify signature
		sigBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid signature encoding")
			return
		}

		pubKeyBytes, err := hex.DecodeString(apiKey.PublicKey)
		if err != nil {
			log.Printf("invalid public key for partner %s: %v", partnerID, err)
			writeError(w, http.StatusInternalServerError, "authentication error")
			return
		}

		if len(pubKeyBytes) != ed25519.PublicKeySize {
			log.Printf("invalid public key size for partner %s", partnerID)
			writeError(w, http.StatusInternalServerError, "authentication error")
			return
		}

		if !ed25519.Verify(pubKeyBytes, []byte(canonical), sigBytes) {
			writeError(w, http.StatusUnauthorized, ErrInvalidSignature.Error())
			return
		}

		// Store request ID (expires in 2 minutes)
		expiresAt := time.Now().Add(2 * time.Minute)
		if err := am.db.StoreRequestID(ctx, requestID, partnerID, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to process request")
			return
		}

		// Add partner context
		ctx = context.WithValue(ctx, ContextKeyPartnerID, partnerID)
		ctx = context.WithValue(ctx, ContextKeyPartner, partner)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":"%s"}`, message)
}
