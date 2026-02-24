package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// mockTransport 用于测试的 mock 传输
type mockTransport struct {
	sendBuf [][]byte
	recvCh  chan []byte
	mu      sync.Mutex
	closed  bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		recvCh: make(chan []byte, 100),
	}
}

func (t *mockTransport) Send(msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return fmt.Errorf("transport closed")
	}
	cp := make([]byte, len(msg))
	copy(cp, msg)
	t.sendBuf = append(t.sendBuf, cp)
	return nil
}

func (t *mockTransport) Receive() ([]byte, error) {
	msg, ok := <-t.recvCh
	if !ok {
		return nil, fmt.Errorf("transport closed")
	}
	return msg, nil
}

func (t *mockTransport) AddResponse(resp *Response) {
	data, _ := json.Marshal(resp)
	t.recvCh <- data
}

func (t *mockTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		close(t.recvCh)
	}
	return nil
}

func TestClientInitialize(t *testing.T) {
	mt := newMockTransport()

	// 预置 initialize 响应
	initResult := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		ServerInfo:      Implementation{Name: "test-server", Version: "0.1"},
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
	}
	resultData, _ := json.Marshal(initResult)
	mt.AddResponse(&Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultData,
	})

	client := NewClient(mt)
	defer client.Close()

	err := client.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	info := client.ServerInfo()
	if info == nil {
		t.Fatal("expected server info")
	}
	if info.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %q", info.ServerInfo.Name)
	}
}

func TestClientListTools(t *testing.T) {
	mt := newMockTransport()

	toolsResult := ToolsListResult{
		Tools: []MCPTool{
			{Name: "calculator", Description: "A calculator tool"},
			{Name: "weather", Description: "Get weather info"},
		},
	}
	resultData, _ := json.Marshal(toolsResult)
	mt.AddResponse(&Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultData,
	})

	client := NewClient(mt)
	defer client.Close()

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "calculator" {
		t.Errorf("expected first tool 'calculator', got %q", tools[0].Name)
	}
}

func TestClientCallTool(t *testing.T) {
	mt := newMockTransport()

	callResult := MCPToolResult{
		Content: []MCPContent{
			{Type: "text", Text: "42"},
		},
	}
	resultData, _ := json.Marshal(callResult)
	mt.AddResponse(&Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultData,
	})

	client := NewClient(mt)
	defer client.Close()

	result, err := client.CallTool(context.Background(), "calculator", map[string]interface{}{
		"expression": "6 * 7",
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	if result.Content[0].Text != "42" {
		t.Errorf("expected '42', got %q", result.Content[0].Text)
	}
}

func TestClientListResources(t *testing.T) {
	mt := newMockTransport()

	resourcesResult := ResourcesListResult{
		Resources: []MCPResource{
			{URI: "file:///data.json", Name: "data", MimeType: "application/json"},
		},
	}
	resultData, _ := json.Marshal(resourcesResult)
	mt.AddResponse(&Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultData,
	})

	client := NewClient(mt)
	defer client.Close()

	resources, err := client.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].URI != "file:///data.json" {
		t.Errorf("expected URI 'file:///data.json', got %q", resources[0].URI)
	}
}

func TestClientCallToolWithError(t *testing.T) {
	mt := newMockTransport()

	mt.AddResponse(&Response{
		JSONRPC: "2.0",
		ID:      1,
		Error: &RPCError{
			Code:    InternalError,
			Message: "Tool execution failed",
		},
	})

	client := NewClient(mt)
	defer client.Close()

	_, err := client.CallTool(context.Background(), "bad-tool", nil)
	if err == nil {
		t.Error("expected error from CallTool")
	}
}
