package mcp

import (
	"encoding/json"
	"testing"
)

func TestNewRequest(t *testing.T) {
	req := NewRequest(1, "tools/list", nil)
	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %q", req.JSONRPC)
	}
	if req.ID != 1 {
		t.Errorf("expected id 1, got %d", req.ID)
	}
	if req.Method != "tools/list" {
		t.Errorf("expected method 'tools/list', got %q", req.Method)
	}
}

func TestEncodeRequest(t *testing.T) {
	req := NewRequest(1, "initialize", InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientInfo:      Implementation{Name: "test", Version: "1.0"},
	})

	data, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc '2.0' in encoded data")
	}
	if parsed["method"] != "initialize" {
		t.Errorf("expected method 'initialize' in encoded data")
	}
}

func TestDecodeResponse(t *testing.T) {
	respJSON := `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","serverInfo":{"name":"test-server","version":"0.1"}}}`

	resp, err := DecodeResponse([]byte(respJSON))
	if err != nil {
		t.Fatalf("DecodeResponse failed: %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %v", resp.Error)
	}
}

func TestDecodeResponseWithError(t *testing.T) {
	respJSON := `{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"Method not found"}}`

	resp, err := DecodeResponse([]byte(respJSON))
	if err != nil {
		t.Fatalf("DecodeResponse failed: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected code %d, got %d", MethodNotFound, resp.Error.Code)
	}
}

func TestDecodeResult(t *testing.T) {
	respJSON := `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"calc","description":"Calculator"}]}}`

	resp, err := DecodeResponse([]byte(respJSON))
	if err != nil {
		t.Fatal(err)
	}

	result, err := DecodeResult[ToolsListResult](resp)
	if err != nil {
		t.Fatalf("DecodeResult failed: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "calc" {
		t.Errorf("expected tool name 'calc', got %q", result.Tools[0].Name)
	}
}

func TestDecodeResultWithRPCError(t *testing.T) {
	respJSON := `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`

	resp, err := DecodeResponse([]byte(respJSON))
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecodeResult[ToolsListResult](resp)
	if err == nil {
		t.Error("expected error from DecodeResult")
	}
}

func TestIDGenerator(t *testing.T) {
	gen := &IDGenerator{}

	id1 := gen.Next()
	id2 := gen.Next()

	if id1 != 1 {
		t.Errorf("expected first id 1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("expected second id 2, got %d", id2)
	}
}

func TestNewNotification(t *testing.T) {
	n := NewNotification("notifications/initialized", nil)
	if n.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0'")
	}
	if n.Method != "notifications/initialized" {
		t.Errorf("expected method 'notifications/initialized'")
	}
}

func TestRPCErrorString(t *testing.T) {
	err := &RPCError{Code: -32601, Message: "Method not found"}
	s := err.Error()
	if s != "RPC error -32601: Method not found" {
		t.Errorf("unexpected error string: %q", s)
	}
}
