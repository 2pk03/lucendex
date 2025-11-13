package xrpl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a WebSocket connection to rippled
type Client struct {
	url          string
	conn         *websocket.Conn
	mu           sync.Mutex
	reconnecting bool
	ctx          context.Context
	cancel       context.CancelFunc
	
	// Channels
	ledgerChan   chan *LedgerResponse
	errorChan    chan error
	
	// Callbacks
	onLedger     func(*LedgerResponse)
	onError      func(error)
	
	// Configuration
	reconnectDelay time.Duration
	maxRetries     int
}

// NewClient creates a new XRPL WebSocket client with default buffer
func NewClient(url string) *Client {
	return NewClientWithBuffer(url, 100)
}

// NewClientWithBuffer creates a new XRPL WebSocket client with custom buffer size
func NewClientWithBuffer(url string, bufferSize int) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Client{
		url:            url,
		ctx:            ctx,
		cancel:         cancel,
		ledgerChan:     make(chan *LedgerResponse, bufferSize),
		errorChan:      make(chan error, 10),
		reconnectDelay: 5 * time.Second,
		maxRetries:     -1, // Infinite retries
	}
}

// Connect establishes WebSocket connection to rippled
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		return errors.New("already connected")
	}
	
	log.Printf("Connecting to rippled at %s", c.url)
	
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	
	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	
	c.conn = conn
	log.Printf("Connected to rippled successfully")
	
	// Start message reader
	go c.readMessages()
	
	return nil
}

// Subscribe subscribes to ledger stream
func (c *Client) Subscribe() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn == nil {
		return errors.New("not connected")
	}
	
	req := SubscribeRequest{
		Command: "subscribe",
		Streams: []string{"ledger"},
	}
	
	if err := c.conn.WriteJSON(req); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	
	log.Printf("Subscribed to ledger stream")
	return nil
}

// readMessages reads messages from WebSocket
func (c *Client) readMessages() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
		
		// Attempt reconnection if not manually closed
		if !c.reconnecting && c.ctx.Err() == nil {
			c.reconnect()
		}
	}()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			c.handleError(fmt.Errorf("read error: %w", err))
			return
		}
		
		c.handleMessage(message)
	}
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(data []byte) {
	// Try parsing as ledger close notification (no "type" field, just ledger data)
	var notification struct {
		LedgerIndex uint64 `json:"ledger_index"`
		LedgerHash  string `json:"ledger_hash"`
		LedgerTime  uint64 `json:"ledger_time"`
		FeeBase     int    `json:"fee_base"`
	}
	
	if err := json.Unmarshal(data, &notification); err == nil {
		if notification.LedgerIndex > 0 && notification.FeeBase > 0 {
			log.Printf("Received ledger close notification for ledger %d", notification.LedgerIndex)
			// Fetch full ledger with transactions
			go c.fetchLedger(notification.LedgerIndex)
			return
		}
	}
	
	// Try parsing as ledger command response
	var cmdResp LedgerCommandResponse
	if err := json.Unmarshal(data, &cmdResp); err == nil {
		if cmdResp.Status == "success" && cmdResp.Result.Validated {
			ledger := &LedgerResponse{
				Type:         "ledgerClosed",
				LedgerIndex:  cmdResp.Result.LedgerIndex,
				LedgerHash:   cmdResp.Result.LedgerHash,
				LedgerTime:   cmdResp.Result.Ledger.CloseTime,
				Validated:    true,
				Transactions: cmdResp.Result.Ledger.Transactions,
				TxnCount:     len(cmdResp.Result.Ledger.Transactions),
			}
			
			// Send to channel
			select {
			case c.ledgerChan <- ledger:
			default:
				log.Printf("Warning: ledger channel full, dropping ledger %d", ledger.LedgerIndex)
			}
			
			// Call callback if set
			if c.onLedger != nil {
				c.onLedger(ledger)
			}
			return
		}
	}
	
	// Try parsing as subscribe response
	var subResp SubscribeResponse
	if err := json.Unmarshal(data, &subResp); err == nil {
		if subResp.Status == "success" {
			log.Printf("Subscription confirmed")
		} else if subResp.Error != "" {
			c.handleError(fmt.Errorf("subscribe error: %s - %s", subResp.Error, subResp.ErrorMessage))
		}
		return
	}
	
	// Log unknown message types for debugging
	log.Printf("Received unknown message type: %s", string(data[:min(100, len(data))]))
}

// handleError processes errors
func (c *Client) handleError(err error) {
	select {
	case c.errorChan <- err:
	default:
		log.Printf("Error channel full, dropping error: %v", err)
	}
	
	if c.onError != nil {
		c.onError(err)
	}
}

// reconnect attempts to reconnect to rippled
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()
	
	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()
	
	retries := 0
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		
		if c.maxRetries >= 0 && retries >= c.maxRetries {
			log.Printf("Max reconnection retries reached")
			return
		}
		
		log.Printf("Reconnecting... (attempt %d)", retries+1)
		
		if err := c.Connect(); err != nil {
			log.Printf("Reconnection failed: %v", err)
			retries++
			time.Sleep(c.reconnectDelay)
			continue
		}
		
		// Resubscribe after reconnection
		if err := c.Subscribe(); err != nil {
			log.Printf("Resubscribe failed: %v", err)
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.mu.Unlock()
			retries++
			time.Sleep(c.reconnectDelay)
			continue
		}
		
		log.Printf("Reconnected successfully")
		return
	}
}

// LedgerChan returns the ledger channel
func (c *Client) LedgerChan() <-chan *LedgerResponse {
	return c.ledgerChan
}

// ErrorChan returns the error channel
func (c *Client) ErrorChan() <-chan error {
	return c.errorChan
}

// OnLedger sets callback for ledger events
func (c *Client) OnLedger(callback func(*LedgerResponse)) {
	c.onLedger = callback
}

// OnError sets callback for errors
func (c *Client) OnError(callback func(error)) {
	c.onError = callback
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	c.cancel()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	
	return nil
}

// fetchLedger requests full ledger data with transactions (async)
func (c *Client) fetchLedger(ledgerIndex uint64) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	
	if conn == nil {
		log.Printf("Not connected, skipping ledger %d", ledgerIndex)
		return
	}
	
	req := map[string]interface{}{
		"command":       "ledger",
		"ledger_index":  ledgerIndex,
		"transactions":  true,
		"expand":        true,
	}
	
	c.mu.Lock()
	err := conn.WriteJSON(req)
	c.mu.Unlock()
	
	if err != nil {
		log.Printf("Failed to request ledger %d: %v", ledgerIndex, err)
	}
}

// FetchLedgerSync fetches a ledger synchronously for backfill
func (c *Client) FetchLedgerSync(ledgerIndex uint64) (*LedgerResponse, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	
	if conn == nil {
		return nil, errors.New("not connected")
	}
	
	req := map[string]interface{}{
		"command":       "ledger",
		"ledger_index":  ledgerIndex,
		"transactions":  true,
		"expand":        true,
	}
	
	c.mu.Lock()
	err := conn.WriteJSON(req)
	c.mu.Unlock()
	
	if err != nil {
		return nil, fmt.Errorf("failed to request ledger: %w", err)
	}
	
	// Wait for response (with timeout)
	timeout := time.After(10 * time.Second)
	
	// Temporary channel for this specific ledger
	responseChan := make(chan *LedgerResponse, 1)
	
	// Set up a temporary callback to catch this ledger
	originalCallback := c.onLedger
	c.onLedger = func(ledger *LedgerResponse) {
		if ledger.LedgerIndex == ledgerIndex {
			select {
			case responseChan <- ledger:
			default:
			}
		}
		if originalCallback != nil {
			originalCallback(ledger)
		}
	}
	defer func() { c.onLedger = originalCallback }()
	
	select {
	case ledger := <-responseChan:
		return ledger, nil
	case <-timeout:
		return nil, fmt.Errorf("timeout waiting for ledger %d", ledgerIndex)
	}
}

// GetServerInfo requests server information via WebSocket (DEPRECATED - use GetServerInfoHTTP)
func (c *Client) GetServerInfo() (*ServerInfoResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn == nil {
		return nil, errors.New("not connected")
	}
	
	req := ServerInfoRequest{
		Command: "server_info",
	}
	
	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send server_info: %w", err)
	}
	
	// Read response (blocking)
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var resp ServerInfoResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &resp, nil
}

// GetServerInfoHTTP requests server information via HTTP RPC (avoids WebSocket conflict)
func GetServerInfoHTTP(rpcURL string) (*ServerInfoResponse, error) {
	reqBody := map[string]interface{}{
		"method": "server_info",
		"params": []interface{}{},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var serverInfo ServerInfoResponse
	if err := json.Unmarshal(body, &serverInfo); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &serverInfo, nil
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
