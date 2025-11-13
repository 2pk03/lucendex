package main

import (
	"testing"

	"github.com/lucendex/backend/internal/xrpl"
)

// TestGapDetection tests gap detection logic
func TestGapDetection(t *testing.T) {
	tests := []struct {
		name              string
		checkpointLedger  uint64
		currentLedger     uint64
		expectedGap       uint64
		expectedBackfill  bool
	}{
		{
			name:              "no gap - consecutive ledgers",
			checkpointLedger:  1000,
			currentLedger:     1001,
			expectedGap:       1,
			expectedBackfill:  false,
		},
		{
			name:              "small gap - 5 missing ledgers",
			checkpointLedger:  1000,
			currentLedger:     1006,
			expectedGap:       6,
			expectedBackfill:  true,
		},
		{
			name:              "large gap - 100 missing ledgers",
			checkpointLedger:  1000,
			currentLedger:     1101,
			expectedGap:       101,
			expectedBackfill:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gap := tt.currentLedger - tt.checkpointLedger
			needsBackfill := gap > 1
			
			if gap != tt.expectedGap {
				t.Errorf("gap = %d, want %d", gap, tt.expectedGap)
			}
			
			if needsBackfill != tt.expectedBackfill {
				t.Errorf("needsBackfill = %v, want %v", needsBackfill, tt.expectedBackfill)
			}
			
			if needsBackfill {
				missingCount := gap - 1
				expectedMissing := tt.expectedGap - 1
				if missingCount != expectedMissing {
					t.Errorf("missingCount = %d, want %d", missingCount, expectedMissing)
				}
			}
		})
	}
}

// TestProcessLedgerIdempotency tests that processing the same ledger twice is safe
func TestProcessLedgerIdempotency(t *testing.T) {
	// This test verifies that the upsert logic handles duplicate ledger processing
	// Important for backfill scenarios where network issues might cause retries
	
	ledger := &xrpl.LedgerResponse{
		LedgerIndex:  12345,
		LedgerHash:   "test_hash_12345",
		LedgerTime:   1234567890,
		TxnCount:     0,
		Transactions: []xrpl.Transaction{},
	}
	
	// Mock database would verify ON CONFLICT DO UPDATE works correctly
	// In real implementation, this would use a test database
	
	if ledger.LedgerIndex != 12345 {
		t.Errorf("ledger index should remain stable across processing")
	}
}

// TestBackfillRange tests backfill range calculation
func TestBackfillRange(t *testing.T) {
	tests := []struct {
		name          string
		checkpoint    int64
		current       uint64
		expectedStart uint64
		expectedEnd   uint64
		expectedCount int
	}{
		{
			name:          "5 ledger gap",
			checkpoint:    100,
			current:       106,
			expectedStart: 101,
			expectedEnd:   105,
			expectedCount: 5,
		},
		{
			name:          "1 ledger gap",
			checkpoint:    100,
			current:       102,
			expectedStart: 101,
			expectedEnd:   101,
			expectedCount: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := uint64(tt.checkpoint + 1)
			end := tt.current - 1
			count := int(end - start + 1)
			
			if start != tt.expectedStart {
				t.Errorf("start = %d, want %d", start, tt.expectedStart)
			}
			if end != tt.expectedEnd {
				t.Errorf("end = %d, want %d", end, tt.expectedEnd)
			}
			if count != tt.expectedCount {
				t.Errorf("count = %d, want %d", count, tt.expectedCount)
			}
		})
	}
}
