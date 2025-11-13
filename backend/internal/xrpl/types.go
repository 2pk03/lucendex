package xrpl

import "encoding/json"

// Core XRPL types for indexer

// LedgerResponse represents a ledger from the subscription stream
type LedgerResponse struct {
	Type          string `json:"type"`
	LedgerIndex   uint64 `json:"ledger_index"`
	LedgerHash    string `json:"ledger_hash"`
	LedgerTime    uint64 `json:"ledger_time"`
	Validated     bool   `json:"validated"`
	Transactions  []Transaction `json:"transactions"`
	TxnCount      int    `json:"txn_count"`
}

// Transaction represents an XRPL transaction
type Transaction struct {
	TransactionType string                 `json:"TransactionType"`
	Account         string                 `json:"Account"`
	Sequence        uint64                 `json:"Sequence,omitempty"`
	Fee             string                 `json:"Fee"`
	Hash            string                 `json:"hash"`
	LedgerIndex     uint64                 `json:"ledger_index,omitempty"`
	Meta            TransactionMeta        `json:"meta,omitempty"`
	MetaData        TransactionMeta        `json:"metaData,omitempty"` // Alternative field name
	// Transaction-specific fields stored as generic map
	Data            map[string]interface{} `json:"-"`
}

// TransactionMeta represents transaction metadata
type TransactionMeta struct {
	TransactionResult string                 `json:"TransactionResult"`
	TransactionIndex  int                    `json:"TransactionIndex"`
	AffectedNodes     []AffectedNode         `json:"AffectedNodes"`
	DeliveredAmount   interface{}            `json:"delivered_amount,omitempty"`
}

// AffectedNode represents a node affected by a transaction
type AffectedNode struct {
	CreatedNode  *NodeChange `json:"CreatedNode,omitempty"`
	ModifiedNode *NodeChange `json:"ModifiedNode,omitempty"`
	DeletedNode  *NodeChange `json:"DeletedNode,omitempty"`
}

// NodeChange represents changes to a ledger node
type NodeChange struct {
	LedgerEntryType string                 `json:"LedgerEntryType"`
	LedgerIndex     string                 `json:"LedgerIndex"`
	NewFields       map[string]interface{} `json:"NewFields,omitempty"`
	FinalFields     map[string]interface{} `json:"FinalFields,omitempty"`
	PreviousFields  map[string]interface{} `json:"PreviousFields,omitempty"`
}

// SubscribeRequest represents a subscription request to rippled
type SubscribeRequest struct {
	Command string   `json:"command"`
	Streams []string `json:"streams"`
}

// SubscribeResponse represents the response to a subscribe request
type SubscribeResponse struct {
	Status        string `json:"status"`
	Type          string `json:"type"`
	LedgerIndex   uint64 `json:"ledger_index,omitempty"`
	LedgerHash    string `json:"ledger_hash,omitempty"`
	Validated     bool   `json:"validated,omitempty"`
	Error         string `json:"error,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// LedgerCommandResponse wraps the ledger command response
type LedgerCommandResponse struct {
	Result struct {
		Ledger struct {
			LedgerIndex   uint64        `json:"ledger_index,string"`
			LedgerHash    string        `json:"ledger_hash"`
			CloseTime     uint64        `json:"close_time"`
			Validated     bool          `json:"validated"`
			Transactions  []Transaction `json:"transactions"`
		} `json:"ledger"`
		LedgerHash  string `json:"ledger_hash"`
		LedgerIndex uint64 `json:"ledger_index"`
		Validated   bool   `json:"validated"`
	} `json:"result"`
	Status string `json:"status"`
}

// ServerInfoRequest requests server information
type ServerInfoRequest struct {
	Command string `json:"command"`
}

// ServerInfoResponse represents server_info response
type ServerInfoResponse struct {
	Result struct {
		Info struct {
			CompleteLedgers string `json:"complete_ledgers"`
			ValidatedLedger struct {
				Seq uint64 `json:"seq"`
			} `json:"validated_ledger"`
			ServerState string `json:"server_state"`
		} `json:"info"`
		Status string `json:"status"`
	} `json:"result"`
	Type string `json:"type,omitempty"`
}

// Amount represents an XRPL amount (XRP or IOU)
type Amount struct {
	Currency string `json:"currency,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	Value    string `json:"value,omitempty"`
	// For XRP, amount is a string number of drops
	Drops string `json:"-"`
}

// UnmarshalJSON handles both XRP (string) and IOU (object) amounts
func (a *Amount) UnmarshalJSON(data []byte) error {
	// Try parsing as string (XRP drops)
	var drops string
	if err := json.Unmarshal(data, &drops); err == nil {
		a.Drops = drops
		a.Currency = "XRP"
		return nil
	}
	
	// Parse as object (IOU)
	type Alias Amount
	aux := &struct{ *Alias }{Alias: (*Alias)(a)}
	return json.Unmarshal(data, aux)
}

// ParseAmount converts Amount to standard format
func (a *Amount) ParseAmount() string {
	if a.Currency == "XRP" || a.Drops != "" {
		return a.Drops
	}
	return a.Value
}

// FormatAsset returns asset in standard format: "XRP" or "CURRENCY.ISSUER"
func FormatAsset(currency, issuer string) string {
	if currency == "XRP" || issuer == "" {
		return "XRP"
	}
	return currency + "." + issuer
}
