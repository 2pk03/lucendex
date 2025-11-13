package router

import (
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

var (
	xrplAddressRegex = regexp.MustCompile(`^r[1-9A-HJ-NP-Za-km-z]{24,34}$`)
	currencyRegex    = regexp.MustCompile(`^[A-Z0-9]{3,40}$`)
)

const (
	MaxAmount = 1e18
)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateQuoteRequest(req *QuoteRequest) error {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}

	maxAmount := decimal.NewFromInt(1e18)
	if req.Amount.GreaterThan(maxAmount) {
		return ErrAmountTooLarge
	}

	if err := v.ValidateAsset(req.In); err != nil {
		return err
	}

	if err := v.ValidateAsset(req.Out); err != nil {
		return err
	}

	if req.In.String() == req.Out.String() {
		return ErrSameAssets
	}

	return nil
}

func (v *Validator) ValidateAsset(asset Asset) error {
	if asset.Currency == "" {
		return ErrInvalidAsset
	}

	if asset.Currency == "XRP" {
		if asset.Issuer != "" {
			return ErrInvalidAsset
		}
		return nil
	}

	if !currencyRegex.MatchString(asset.Currency) {
		return ErrInvalidAsset
	}

	if asset.Issuer == "" {
		return ErrInvalidAsset
	}

	if !v.IsValidXRPLAddress(asset.Issuer) {
		return ErrInvalidAddress
	}

	return nil
}

func (v *Validator) IsValidXRPLAddress(address string) bool {
	if address == "" {
		return false
	}

	address = strings.TrimSpace(address)

	if !xrplAddressRegex.MatchString(address) {
		return false
	}

	return true
}
