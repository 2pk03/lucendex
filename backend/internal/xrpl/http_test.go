package xrpl

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetServerInfoHTTP(t *testing.T) {
	tests := []struct {
		name       string
		serverResp string
		wantErr    bool
		wantSeq    uint64
	}{
		{
			name: "valid server_info response",
			serverResp: `{
				"result": {
					"info": {
						"validated_ledger": {
							"seq": 12345
						}
					},
					"status": "success"
				}
			}`,
			wantErr: false,
			wantSeq: 12345,
		},
		{
			name:       "invalid JSON",
			serverResp: `{invalid json}`,
			wantErr:    true,
			wantSeq:    0,
		},
		{
			name: "error response",
			serverResp: `{
				"result": {
					"error": "internal",
					"status": "error"
				}
			}`,
			wantErr: false, // We get response, even if error
			wantSeq: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.serverResp))
			}))
			defer server.Close()
			
			info, err := GetServerInfoHTTP(server.URL)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("GetServerInfoHTTP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && info != nil {
				if info.Result.Info.ValidatedLedger.Seq != tt.wantSeq {
					t.Errorf("seq = %v, want %v", info.Result.Info.ValidatedLedger.Seq, tt.wantSeq)
				}
			}
		})
	}
}

func TestGetServerInfoHTTP_NetworkError(t *testing.T) {
	// Test with invalid URL (connection refused)
	_, err := GetServerInfoHTTP("http://localhost:99999")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}
