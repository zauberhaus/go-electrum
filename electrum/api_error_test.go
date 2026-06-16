package electrum_test

import (
	"encoding/json"
	"testing"

	"github.com/zauberhaus/go-electrum/electrum"
)

func TestAPIError_UnmarshalJSON_Object(t *testing.T) {
	var e electrum.APIError
	if err := json.Unmarshal([]byte(`{"code":-32000,"message":"boom"}`), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Code != -32000 || e.Message != "boom" {
		t.Fatalf("got %+v", e)
	}
}

func TestAPIError_UnmarshalJSON_String(t *testing.T) {
	// Some ElectrumX servers return the error as a bare string.
	var e electrum.APIError
	if err := json.Unmarshal([]byte(`"Too many history entries"`), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Code != 0 || e.Message != "Too many history entries" {
		t.Fatalf("got %+v", e)
	}
}

// TestAPIError_UnmarshalJSON_InResponse verifies the string form is accepted
// when it appears as the "error" field of a JSON-RPC response.
func TestAPIError_UnmarshalJSON_InResponse(t *testing.T) {
	var resp struct {
		ID    uint64             `json:"id"`
		Error *electrum.APIError `json:"error,omitempty"`
	}
	raw := []byte(`{"id":3,"jsonrpc":"2.0","error":"Too many history entries"}`)
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error == nil || resp.Error.Message != "Too many history entries" {
		t.Fatalf("got %+v", resp.Error)
	}
}
