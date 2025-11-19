package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockDB struct {
	partner   *Partner
	apiKey    *APIKey
	requestID map[string]bool
	err       error
}

func (m *mockDB) GetPartnerByID(ctx context.Context, partnerID uuid.UUID) (*Partner, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.partner, nil
}

func (m *mockDB) GetAPIKeyByPublicKey(ctx context.Context, publicKey string) (*APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.apiKey, nil
}

func (m *mockDB) GetActiveAPIKey(ctx context.Context, partnerID uuid.UUID) (*APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.apiKey, nil
}

func (m *mockDB) CheckRequestID(ctx context.Context, requestID uuid.UUID, partnerID uuid.UUID) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.requestID[requestID.String()], nil
}

func (m *mockDB) StoreRequestID(ctx context.Context, requestID uuid.UUID, partnerID uuid.UUID, expiresAt time.Time) error {
	if m.requestID == nil {
		m.requestID = make(map[string]bool)
	}
	m.requestID[requestID.String()] = true
	return m.err
}

func (m *mockDB) GetPartnerUsage(ctx context.Context, partnerID uuid.UUID, month string) (*UsageResponse, error) {
	return &UsageResponse{}, m.err
}

func (m *mockDB) StoreQuoteRegistry(ctx context.Context, registry *QuoteRegistry) error {
	return m.err
}

func (m *mockDB) GetIndexerLag(ctx context.Context) (int, error) {
	return 0, m.err
}

func (m *mockDB) UpdateNetworkLedger(ctx context.Context, ledger uint32) error {
	return m.err
}

func TestAuthMiddleware_ValidSignature(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	partnerID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := []byte(`{"test":"data"}`)

	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("POST\n/partner/v1/quote\n\n%x\n%s",
		bodyHash, timestamp)

	signature := ed25519.Sign(privKey, []byte(canonical))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	apiKey := &APIKey{
		ID:        uuid.New(),
		PartnerID: partnerID,
		PublicKey: hex.EncodeToString(pubKey),
	}

	db := &mockDB{
		partner: &Partner{
			ID:        partnerID,
			Name:      "Test Partner",
			Plan:      "pro",
			RouterBps: 20,
			Status:    "active",
		},
		apiKey:    apiKey,
		requestID: make(map[string]bool),
	}

	auth := NewAuthMiddleware(db)

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("POST", "/partner/v1/quote", bytes.NewReader(body))
	req.Header.Set("X-Partner-Id", partnerID.String())
	req.Header.Set("X-Request-Id", requestID.String())
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signatureB64)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_InvalidSignature(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	_, wrongPrivKey, _ := ed25519.GenerateKey(rand.Reader)

	partnerID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := []byte(`{"test":"data"}`)

	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("POST\n/partner/v1/quote\n\n%x\n%s",
		bodyHash, timestamp)

	signature := ed25519.Sign(wrongPrivKey, []byte(canonical))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	db := &mockDB{
		partner: &Partner{
			ID:        partnerID,
			Name:      "Test Partner",
			Plan:      "pro",
			RouterBps: 20,
			Status:    "active",
		},
		apiKey: &APIKey{
			ID:        uuid.New(),
			PartnerID: partnerID,
			PublicKey: hex.EncodeToString(pubKey),
		},
		requestID: make(map[string]bool),
	}

	auth := NewAuthMiddleware(db)

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/partner/v1/quote", bytes.NewReader(body))
	req.Header.Set("X-Partner-Id", partnerID.String())
	req.Header.Set("X-Request-Id", requestID.String())
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signatureB64)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ExpiredTimestamp(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	partnerID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	body := []byte(`{"test":"data"}`)

	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("POST\n/partner/v1/quote\n\n%x\n%s",
		bodyHash, timestamp)

	signature := ed25519.Sign(privKey, []byte(canonical))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	db := &mockDB{
		partner: &Partner{
			ID:        partnerID,
			Name:      "Test Partner",
			Plan:      "pro",
			RouterBps: 20,
			Status:    "active",
		},
		apiKey: &APIKey{
			ID:        uuid.New(),
			PartnerID: partnerID,
			PublicKey: hex.EncodeToString(pubKey),
		},
		requestID: make(map[string]bool),
	}

	auth := NewAuthMiddleware(db)

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/partner/v1/quote", bytes.NewReader(body))
	req.Header.Set("X-Partner-Id", partnerID.String())
	req.Header.Set("X-Request-Id", requestID.String())
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signatureB64)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ReplayAttack(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	partnerID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := []byte(`{"test":"data"}`)

	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("POST\n/partner/v1/quote\n\n%x\n%s",
		bodyHash, timestamp)

	signature := ed25519.Sign(privKey, []byte(canonical))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	db := &mockDB{
		partner: &Partner{
			ID:        partnerID,
			Name:      "Test Partner",
			Plan:      "pro",
			RouterBps: 20,
			Status:    "active",
		},
		apiKey: &APIKey{
			ID:        uuid.New(),
			PartnerID: partnerID,
			PublicKey: hex.EncodeToString(pubKey),
		},
		requestID: map[string]bool{
			requestID.String(): true,
		},
	}

	auth := NewAuthMiddleware(db)

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/partner/v1/quote", bytes.NewReader(body))
	req.Header.Set("X-Partner-Id", partnerID.String())
	req.Header.Set("X-Request-Id", requestID.String())
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signatureB64)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for replay attack, got %d", rec.Code)
	}
}

func TestAuthMiddleware_SuspendedPartner(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	partnerID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := []byte(`{"test":"data"}`)

	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("POST\n/partner/v1/quote\n\n%x\n%s",
		bodyHash, timestamp)

	signature := ed25519.Sign(privKey, []byte(canonical))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	db := &mockDB{
		partner: &Partner{
			ID:        partnerID,
			Name:      "Test Partner",
			Plan:      "pro",
			RouterBps: 20,
			Status:    "suspended",
		},
		apiKey: &APIKey{
			ID:        uuid.New(),
			PartnerID: partnerID,
			PublicKey: hex.EncodeToString(pubKey),
		},
		requestID: make(map[string]bool),
	}

	auth := NewAuthMiddleware(db)

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/partner/v1/quote", bytes.NewReader(body))
	req.Header.Set("X-Partner-Id", partnerID.String())
	req.Header.Set("X-Request-Id", requestID.String())
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signatureB64)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for suspended partner, got %d", rec.Code)
	}
}
