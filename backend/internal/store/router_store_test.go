package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRouterStore_GetRateLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := &RouterStore{db: db}
	ctx := context.Background()

	partnerID := "partner-123"
	bucket := int64(12345)

	t.Run("found", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"partner_id", "bucket", "count", "expires_at"}).
			AddRow(partnerID, bucket, 5, time.Now().Add(1*time.Minute))

		mock.ExpectQuery("SELECT (.+) FROM metering.rate_limits").
			WithArgs(partnerID, bucket).
			WillReturnRows(rows)

		rl, err := store.GetRateLimit(ctx, partnerID, bucket)
		if err != nil {
			t.Errorf("GetRateLimit() error = %v", err)
		}
		if rl == nil {
			t.Fatal("Expected rate limit, got nil")
		}
		if rl.Count != 5 {
			t.Errorf("Count = %d, want 5", rl.Count)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM metering.rate_limits").
			WithArgs(partnerID, bucket).
			WillReturnError(sql.ErrNoRows)

		rl, err := store.GetRateLimit(ctx, partnerID, bucket)
		if err != nil {
			t.Errorf("GetRateLimit() error = %v", err)
		}
		if rl != nil {
			t.Errorf("Expected nil, got %v", rl)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

func TestRouterStore_UpsertRateLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := &RouterStore{db: db}
	ctx := context.Background()

	rl := &RateLimit{
		PartnerID: "partner-123",
		Bucket:    12345,
		Count:     5,
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	mock.ExpectExec("INSERT INTO metering.rate_limits").
		WithArgs(rl.PartnerID, rl.Bucket, rl.Count, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertRateLimit(ctx, rl); err != nil {
		t.Errorf("UpsertRateLimit() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestRouterStore_IsQuoteUsed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := &RouterStore{db: db}
	ctx := context.Background()
	quoteHash := []byte("test-hash-123")

	t.Run("quote exists", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(quoteHash).
			WillReturnRows(rows)

		used, err := store.IsQuoteUsed(ctx, quoteHash)
		if err != nil {
			t.Errorf("IsQuoteUsed() error = %v", err)
		}
		if !used {
			t.Error("Expected true, got false")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})

	t.Run("quote not found", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(quoteHash).
			WillReturnRows(rows)

		used, err := store.IsQuoteUsed(ctx, quoteHash)
		if err != nil {
			t.Errorf("IsQuoteUsed() error = %v", err)
		}
		if used {
			t.Error("Expected false, got true")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

func TestRouterStore_MarkQuoteUsed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := &RouterStore{db: db}
	ctx := context.Background()

	partnerID := "partner-123"
	txHash := "tx-hash-abc"
	uq := &UsedQuote{
		QuoteHash:   []byte("quote-hash"),
		LedgerIndex: 12345,
		PartnerID:   &partnerID,
		TxHash:      &txHash,
		RouteSummary: map[string]interface{}{
			"in": "XRP",
		},
	}

	mock.ExpectExec("INSERT INTO metering.used_quotes").
		WithArgs(uq.QuoteHash, uq.LedgerIndex, uq.PartnerID, uq.TxHash, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.MarkQuoteUsed(ctx, uq); err != nil {
		t.Errorf("MarkQuoteUsed() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestRouterStore_LogAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := &RouterStore{db: db}
	ctx := context.Background()

	partnerID := "partner-123"
	durationMs := 150
	errorCode := "TEST_ERROR"

	log := &RouterAuditLog{
		Event:      "quote_request",
		PartnerID:  &partnerID,
		Severity:   "info",
		DurationMs: &durationMs,
		Outcome:    "success",
		Metadata: map[string]interface{}{
			"pair": "XRP-USD",
		},
		ErrorCode: &errorCode,
	}

	mock.ExpectExec("INSERT INTO metering.router_audit").
		WithArgs(log.Event, log.PartnerID, log.Severity, log.DurationMs, log.Outcome, sqlmock.AnyArg(), log.ErrorCode).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.LogAudit(ctx, log); err != nil {
		t.Errorf("LogAudit() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestRouterStore_SQLInjectionPrevention(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := &RouterStore{db: db}
	ctx := context.Background()

	maliciousPartnerID := "' OR '1'='1"
	
	mock.ExpectQuery("SELECT (.+) FROM metering.rate_limits").
		WithArgs(maliciousPartnerID, int64(123)).
		WillReturnError(sql.ErrNoRows)

	_, err = store.GetRateLimit(ctx, maliciousPartnerID, 123)
	if err != nil && err != sql.ErrNoRows {
		t.Logf("Parameterized query safely handled malicious input: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}
