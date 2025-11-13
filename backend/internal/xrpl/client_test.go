package xrpl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Mock WebSocket server for testing
type mockWSServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	received []string
	toSend   []string
	t        *testing.T
}

func newMockWSServer(t *testing.T) *mockWSServer {
	m := &mockWSServer{
		t:        t,
		upgrader: websocket.Upgrader{},
		received: []string{},
		toSend:   []string{},
	}
	
	m.server = httptest.NewServer(http.HandlerFunc(m.handler))
	return m
}

func (m *mockWSServer) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.t.Errorf("upgrade error: %v", err)
		return
	}
	defer conn.Close()
	
	// Send queued messages
	for _, msg := range m.toSend {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			m.t.Errorf("write error: %v", err)
			return
		}
	}
	
	// Read messages from client
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		m.received = append(m.received, string(message))
	}
}

func (m *mockWSServer) URL() string {
	return "ws" + strings.TrimPrefix(m.server.URL, "http")
}

func (m *mockWSServer) Close() {
	m.server.Close()
}

func TestNewClient(t *testing.T) {
	url := "ws://localhost:6006"
	client := NewClient(url)
	
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	
	if client.url != url {
		t.Errorf("url = %v, want %v", client.url, url)
	}
	
	if client.reconnectDelay != 5*time.Second {
		t.Errorf("reconnectDelay = %v, want %v", client.reconnectDelay, 5*time.Second)
	}
	
	if client.maxRetries != -1 {
		t.Errorf("maxRetries = %v, want -1", client.maxRetries)
	}
	
	if client.ledgerChan == nil {
		t.Error("ledgerChan is nil")
	}
	
	if client.errorChan == nil {
		t.Error("errorChan is nil")
	}
}

func TestClient_Connect(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*Client, *mockWSServer)
		wantErr bool
	}{
		{
			name: "successful connection",
			setup: func() (*Client, *mockWSServer) {
				server := newMockWSServer(t)
				client := NewClient(server.URL())
				return client, server
			},
			wantErr: false,
		},
		{
			name: "already connected",
			setup: func() (*Client, *mockWSServer) {
				server := newMockWSServer(t)
				client := NewClient(server.URL())
				// Connect once
				if err := client.Connect(); err != nil {
					t.Fatalf("initial connection failed: %v", err)
				}
				return client, server
			},
			wantErr: true, // Second connection should fail
		},
		{
			name: "invalid URL",
			setup: func() (*Client, *mockWSServer) {
				client := NewClient("ws://invalid-url-that-does-not-exist:9999")
				return client, nil
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := tt.setup()
			if server != nil {
				defer server.Close()
			}
			defer client.Close()
			
			err := client.Connect()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Subscribe(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*Client, *mockWSServer)
		wantErr bool
	}{
		{
			name: "successful subscribe",
			setup: func() (*Client, *mockWSServer) {
				server := newMockWSServer(t)
				// Queue subscribe response
				server.toSend = []string{`{"status":"success","type":"response"}`}
				client := NewClient(server.URL())
				if err := client.Connect(); err != nil {
					t.Fatalf("connection failed: %v", err)
				}
				time.Sleep(100 * time.Millisecond) // Let connection establish
				return client, server
			},
			wantErr: false,
		},
		{
			name: "subscribe without connection",
			setup: func() (*Client, *mockWSServer) {
				client := NewClient("ws://localhost:6006")
				return client, nil
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := tt.setup()
			if server != nil {
				defer server.Close()
			}
			defer client.Close()
			
			err := client.Subscribe()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Subscribe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_LedgerCallback(t *testing.T) {
	server := newMockWSServer(t)
	defer server.Close()
	
	// Queue ledger command response (new format)
	ledgerMsg := `{
		"status": "success",
		"result": {
			"ledger_index": 12345,
			"ledger_hash": "ABC123",
			"validated": true,
			"ledger": {
				"ledger_index": "12345",
				"ledger_hash": "ABC123",
				"close_time": 741234567,
				"validated": true,
				"transactions": []
			}
		}
	}`
	server.toSend = []string{ledgerMsg}
	
	client := NewClient(server.URL())
	defer client.Close()
	
	// Set up callback
	called := false
	var receivedLedger *LedgerResponse
	client.OnLedger(func(lr *LedgerResponse) {
		called = true
		receivedLedger = lr
	})
	
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	
	// Wait for callback
	time.Sleep(200 * time.Millisecond)
	
	if !called {
		t.Error("OnLedger callback was not called")
	}
	
	if receivedLedger == nil {
		t.Fatal("receivedLedger is nil")
	}
	
	if receivedLedger.LedgerIndex != 12345 {
		t.Errorf("LedgerIndex = %v, want 12345", receivedLedger.LedgerIndex)
	}
	
	if receivedLedger.LedgerHash != "ABC123" {
		t.Errorf("LedgerHash = %v, want ABC123", receivedLedger.LedgerHash)
	}
}

func TestClient_LedgerChannel(t *testing.T) {
	server := newMockWSServer(t)
	defer server.Close()
	
	// Queue ledger command response (new format)
	ledgerMsg := `{
		"status": "success",
		"result": {
			"ledger_index": 67890,
			"ledger_hash": "DEF456",
			"validated": true,
			"ledger": {
				"ledger_index": "67890",
				"ledger_hash": "DEF456",
				"close_time": 741234567,
				"validated": true,
				"transactions": []
			}
		}
	}`
	server.toSend = []string{ledgerMsg}
	
	client := NewClient(server.URL())
	defer client.Close()
	
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	
	// Read from channel
	select {
	case ledger := <-client.LedgerChan():
		if ledger.LedgerIndex != 67890 {
			t.Errorf("LedgerIndex = %v, want 67890", ledger.LedgerIndex)
		}
		if ledger.LedgerHash != "DEF456" {
			t.Errorf("LedgerHash = %v, want DEF456", ledger.LedgerHash)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for ledger on channel")
	}
}

func TestClient_ErrorCallback(t *testing.T) {
	server := newMockWSServer(t)
	
	// Queue an error response
	errorMsg := `{
		"status": "error",
		"error": "invalidParams",
		"error_message": "Invalid parameters"
	}`
	server.toSend = []string{errorMsg}
	
	client := NewClient(server.URL())
	defer client.Close()
	
	// Set up error callback
	errorCalled := false
	var receivedError error
	client.OnError(func(err error) {
		errorCalled = true
		receivedError = err
	})
	
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	
	// Close server to trigger read error
	server.Close()
	
	// Wait for error callback
	time.Sleep(200 * time.Millisecond)
	
	if !errorCalled {
		t.Error("OnError callback was not called")
	}
	
	if receivedError == nil {
		t.Error("receivedError is nil")
	}
}

func TestClient_Close(t *testing.T) {
	server := newMockWSServer(t)
	defer server.Close()
	
	client := NewClient(server.URL())
	
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	
	// Close should not error
	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	
	// Second close should not error
	if err := client.Close(); err != nil {
		t.Errorf("second Close() error = %v, want nil", err)
	}
}

func TestClient_handleMessage(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		expectLedger  bool
		expectError   bool
	}{
		{
			name: "ledger notification (triggers async fetch)",
			message: `{
				"ledger_index": 12345,
				"ledger_hash": "ABC",
				"ledger_time": 123456,
				"fee_base": 10
			}`,
			expectLedger: false, // Async - doesn't immediately send to channel
			expectError:  false,
		},
		{
			name: "subscribe success",
			message: `{
				"status": "success",
				"type": "response"
			}`,
			expectLedger: false,
			expectError:  false,
		},
		{
			name: "subscribe error",
			message: `{
				"status": "error",
				"error": "unknownStream",
				"error_message": "Unknown stream"
			}`,
			expectLedger: false,
			expectError:  true,
		},
		{
			name:         "invalid JSON",
			message:      `{invalid json}`,
			expectLedger: false,
			expectError:  false, // handleMessage logs but doesn't error on unknown types
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("ws://localhost:6006")
			
			ledgerReceived := false
			errorReceived := false
			
			client.OnLedger(func(lr *LedgerResponse) {
				ledgerReceived = true
			})
			
			client.OnError(func(err error) {
				errorReceived = true
			})
			
			client.handleMessage([]byte(tt.message))
			
			if ledgerReceived != tt.expectLedger {
				t.Errorf("ledgerReceived = %v, want %v", ledgerReceived, tt.expectLedger)
			}
			
			if errorReceived != tt.expectError {
				t.Errorf("errorReceived = %v, want %v", errorReceived, tt.expectError)
			}
		})
	}
}

// Test helper function
func TestMin(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{
			name: "a < b",
			a:    5,
			b:    10,
			want: 5,
		},
		{
			name: "a > b",
			a:    10,
			b:    5,
			want: 5,
		},
		{
			name: "a == b",
			a:    7,
			b:    7,
			want: 7,
		},
		{
			name: "negative numbers",
			a:    -5,
			b:    -10,
			want: -10,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := min(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("min() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark for message handling
func BenchmarkClient_handleMessage(b *testing.B) {
	client := NewClient("ws://localhost:6006")
	message := []byte(`{
		"type": "ledgerClosed",
		"ledger_index": 12345,
		"ledger_hash": "ABC123",
		"ledger_time": 741234567,
		"validated": true,
		"txn_count": 10
	}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.handleMessage(message)
	}
}

// Benchmark for JSON unmarshaling
func BenchmarkLedgerResponse_Unmarshal(b *testing.B) {
	data := []byte(`{
		"type": "ledgerClosed",
		"ledger_index": 12345,
		"ledger_hash": "ABC123",
		"ledger_time": 741234567,
		"validated": true,
		"txn_count": 10
	}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var lr LedgerResponse
		json.Unmarshal(data, &lr)
	}
}
