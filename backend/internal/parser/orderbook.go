package parser

import (
	"fmt"
	"strconv"

	"github.com/lucendex/backend/internal/store"
)

// OrderbookParser parses orderbook-related transactions
type OrderbookParser struct{}

// NewOrderbookParser creates a new orderbook parser
func NewOrderbookParser() *OrderbookParser {
	return &OrderbookParser{}
}

// ParseTransaction parses an orderbook transaction and returns an Offer if applicable
func (p *OrderbookParser) ParseTransaction(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string) (*store.Offer, error) {
	txType, ok := tx["TransactionType"].(string)
	if !ok {
		return nil, fmt.Errorf("missing TransactionType")
	}
	
	// Only process orderbook-related transaction types
	switch txType {
	case "OfferCreate":
		return p.parseOfferCreate(tx, ledgerIndex, ledgerHash)
	case "OfferCancel":
		// OfferCancel is handled differently (returns cancel info, not an offer)
		return nil, nil
	default:
		return nil, nil // Not an orderbook transaction
	}
}

// ParseOfferCancel extracts cancel information from OfferCancel transaction
func (p *OrderbookParser) ParseOfferCancel(tx map[string]interface{}) (ownerAccount string, offerSequence int64, err error) {
	txType, ok := tx["TransactionType"].(string)
	if !ok || txType != "OfferCancel" {
		return "", 0, fmt.Errorf("not an OfferCancel transaction")
	}
	
	// Extract account
	account, ok := tx["Account"].(string)
	if !ok {
		return "", 0, fmt.Errorf("missing Account field")
	}
	
	// Extract offer sequence
	sequence, ok := tx["OfferSequence"]
	if !ok {
		return "", 0, fmt.Errorf("missing OfferSequence field")
	}
	
	// Handle both float64 and int types
	var seq int64
	switch v := sequence.(type) {
	case float64:
		seq = int64(v)
	case int:
		seq = int64(v)
	case int64:
		seq = v
	default:
		return "", 0, fmt.Errorf("invalid OfferSequence type")
	}
	
	return account, seq, nil
}

// parseOfferCreate handles OfferCreate transaction
func (p *OrderbookParser) parseOfferCreate(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string) (*store.Offer, error) {
	// Extract account (owner) - required for all offers
	account, ok := tx["Account"].(string)
	if !ok {
		return p.createInvalidOffer(tx, ledgerIndex, ledgerHash, "missing Account field"), nil
	}
	
	// Extract sequence - required for all offers
	sequence, ok := tx["Sequence"]
	if !ok {
		return p.createInvalidOffer(tx, ledgerIndex, ledgerHash, "missing Sequence field"), nil
	}
	
	offerSeq := int64(0)
	switch v := sequence.(type) {
	case float64:
		offerSeq = int64(v)
	case int:
		offerSeq = int64(v)
	case int64:
		offerSeq = v
	default:
		return p.createInvalidOffer(tx, ledgerIndex, ledgerHash, "invalid Sequence type"), nil
	}
	
	// Extract TakerPays (what taker pays = what maker receives)
	takerPays, ok := tx["TakerPays"]
	if !ok {
		return p.createInvalidOfferWithSeq(account, offerSeq, ledgerIndex, ledgerHash, "missing TakerPays field"), nil
	}
	
	// Extract TakerGets (what taker gets = what maker pays)
	takerGets, ok := tx["TakerGets"]
	if !ok {
		return p.createInvalidOfferWithSeq(account, offerSeq, ledgerIndex, ledgerHash, "missing TakerGets field"), nil
	}
	
	// Parse amounts
	paysAsset, paysAmount, err := parseAmount(takerPays)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TakerPays: %w", err)
	}
	
	getsAsset, getsAmount, err := parseAmount(takerGets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TakerGets: %w", err)
	}
	
	// Determine side and calculate price
	// If maker is selling getsAsset for paysAsset:
	// - Side is 'ask' (offering to sell)
	// - Base = getsAsset, Quote = paysAsset
	// - Price = paysAmount / getsAmount
	baseAsset := getsAsset
	quoteAsset := paysAsset
	side := "ask"
	amount := getsAmount
	
	// Calculate price (quote/base)
	price, err := calculatePrice(paysAmount, getsAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate price: %w", err)
	}
	
	// Extract expiration (optional)
	var expiration *int64
	if exp, ok := tx["Expiration"]; ok {
		if expFloat, ok := exp.(float64); ok {
			expInt := int64(expFloat)
			expiration = &expInt
		}
	}
	
	offer := &store.Offer{
		BaseAsset:     baseAsset,
		QuoteAsset:    quoteAsset,
		Side:          side,
		Price:         price,
		Amount:        amount,
		OfferSequence: offerSeq,
		OwnerAccount:  account,
		Expiration:    expiration,
		LedgerIndex:   int64(ledgerIndex),
		LedgerHash:    ledgerHash,
		Status:        "active",
	}
	
	return offer, nil
}

// createInvalidOffer creates an offer record for unparseable transaction (no account/sequence)
func (p *OrderbookParser) createInvalidOffer(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string, reason string) *store.Offer {
	return &store.Offer{
		BaseAsset:     "UNKNOWN",
		QuoteAsset:    "UNKNOWN",
		Side:          "bid",
		Price:         "1",
		Amount:        "1",
		OfferSequence: 0,
		OwnerAccount:  "UNKNOWN",
		LedgerIndex:   int64(ledgerIndex),
		LedgerHash:    ledgerHash,
		Status:        "invalid_parse",
		Meta: map[string]interface{}{
			"error":  reason,
			"tx_raw": tx,
		},
	}
}

// createInvalidOfferWithSeq creates an offer record for unparseable transaction (has account/sequence)
func (p *OrderbookParser) createInvalidOfferWithSeq(account string, sequence int64, ledgerIndex uint64, ledgerHash string, reason string) *store.Offer {
	return &store.Offer{
		BaseAsset:     "UNKNOWN",
		QuoteAsset:    "UNKNOWN",
		Side:          "bid",
		Price:         "1",
		Amount:        "1",
		OfferSequence: sequence,
		OwnerAccount:  account,
		LedgerIndex:   int64(ledgerIndex),
		LedgerHash:    ledgerHash,
		Status:        "invalid_parse",
		Meta: map[string]interface{}{
			"error": reason,
		},
	}
}

// calculatePrice computes price as quote/base
func calculatePrice(quoteAmount, baseAmount string) (string, error) {
	// For simplicity, return a ratio string
	// In production, use decimal library for precision
	quoteFloat, err := strconv.ParseFloat(quoteAmount, 64)
	if err != nil {
		return "", fmt.Errorf("invalid quote amount: %w", err)
	}
	
	baseFloat, err := strconv.ParseFloat(baseAmount, 64)
	if err != nil {
		return "", fmt.Errorf("invalid base amount: %w", err)
	}
	
	if baseFloat == 0 {
		return "", fmt.Errorf("base amount cannot be zero")
	}
	
	price := quoteFloat / baseFloat
	return fmt.Sprintf("%.8f", price), nil
}
