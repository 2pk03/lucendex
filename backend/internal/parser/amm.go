package parser

import (
	"fmt"

	"github.com/lucendex/backend/internal/store"
	"github.com/lucendex/backend/internal/xrpl"
)

// AMMParser parses AMM-related transactions
type AMMParser struct{}

// NewAMMParser creates a new AMM parser
func NewAMMParser() *AMMParser {
	return &AMMParser{}
}

// ParseTransaction parses an AMM transaction and returns an AMMPool if applicable
func (p *AMMParser) ParseTransaction(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string) (*store.AMMPool, error) {
	txType, ok := tx["TransactionType"].(string)
	if !ok {
		return nil, fmt.Errorf("missing TransactionType")
	}
	
	// Only process AMM-related transaction types
	switch txType {
	case "AMMCreate":
		return p.parseAMMCreate(tx, ledgerIndex, ledgerHash)
	case "AMMDeposit":
		return p.parseAMMDeposit(tx, ledgerIndex, ledgerHash)
	case "AMMWithdraw":
		return p.parseAMMWithdraw(tx, ledgerIndex, ledgerHash)
	default:
		return nil, nil // Not an AMM transaction
	}
}

// parseAMMCreate handles AMMCreate transaction
func (p *AMMParser) parseAMMCreate(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string) (*store.AMMPool, error) {
	// Extract account
	account, ok := tx["Account"].(string)
	if !ok {
		return nil, fmt.Errorf("missing Account field")
	}
	
	// Extract Amount (asset1)
	amount, ok := tx["Amount"]
	if !ok {
		return nil, fmt.Errorf("missing Amount field")
	}
	
	asset1, asset1Reserve, err := parseAmount(amount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Amount: %w", err)
	}
	
	// Extract Amount2 (asset2)
	amount2, ok := tx["Amount2"]
	if !ok {
		return nil, fmt.Errorf("missing Amount2 field")
	}
	
	asset2, asset2Reserve, err := parseAmount(amount2)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Amount2: %w", err)
	}
	
	// Extract trading fee (optional, default 0)
	tradingFee := 0
	if fee, ok := tx["TradingFee"].(float64); ok {
		tradingFee = int(fee)
	}
	
	// Generate LP token identifier (simplified - real implementation would parse from metadata)
	lpToken := fmt.Sprintf("LP_%s_%s", asset1, asset2)
	
	pool := &store.AMMPool{
		Asset1:        asset1,
		Asset2:        asset2,
		Account:       account,
		LPToken:       lpToken,
		Asset1Reserve: asset1Reserve,
		Asset2Reserve: asset2Reserve,
		TradingFee:    tradingFee,
		LedgerIndex:   int64(ledgerIndex),
		LedgerHash:    ledgerHash,
	}
	
	return pool, nil
}

// parseAMMDeposit handles AMMDeposit transaction
func (p *AMMParser) parseAMMDeposit(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string) (*store.AMMPool, error) {
	// For deposits, we need to update the pool reserves
	// This requires fetching the AMM account from metadata
	// For now, return nil (will be implemented with metadata parsing)
	return nil, nil
}

// parseAMMWithdraw handles AMMWithdraw transaction
func (p *AMMParser) parseAMMWithdraw(tx map[string]interface{}, ledgerIndex uint64, ledgerHash string) (*store.AMMPool, error) {
	// For withdrawals, we need to update the pool reserves
	// This requires fetching the AMM account from metadata
	// For now, return nil (will be implemented with metadata parsing)
	return nil, nil
}

// parseAmount extracts asset identifier and amount from XRPL Amount field
func parseAmount(amount interface{}) (asset string, value string, err error) {
	// Handle XRP (string)
	if str, ok := amount.(string); ok {
		return "XRP", str, nil
	}
	
	// Handle IOU (object)
	if obj, ok := amount.(map[string]interface{}); ok {
		currency, ok1 := obj["currency"].(string)
		issuer, ok2 := obj["issuer"].(string)
		value, ok3 := obj["value"].(string)
		
		if !ok1 || !ok2 || !ok3 {
			return "", "", fmt.Errorf("invalid IOU amount object")
		}
		
		asset = xrpl.FormatAsset(currency, issuer)
		return asset, value, nil
	}
	
	return "", "", fmt.Errorf("invalid amount type")
}
