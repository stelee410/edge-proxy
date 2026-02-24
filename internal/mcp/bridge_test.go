package mcp

import (
	"encoding/json"
	"testing"
)

func TestMCPToolsToLLMTools(t *testing.T) {
	mcpTools := []MCPTool{
		{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
		{
			Name:        "list_dir",
			Description: "List directory",
		},
	}

	llmTools := MCPToolsToLLMTools("filesystem", mcpTools)

	if len(llmTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(llmTools))
	}
	if llmTools[0].Name != "mcp_filesystem__read_file" {
		t.Errorf("expected name 'mcp_filesystem__read_file', got %q", llmTools[0].Name)
	}
	if llmTools[1].Name != "mcp_filesystem__list_dir" {
		t.Errorf("expected name 'mcp_filesystem__list_dir', got %q", llmTools[1].Name)
	}
}

func TestIsMCPTool(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"mcp_server__tool", true},
		{"mcp_fs__read", true},
		{"local-skill", false},
		{"mcptool", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsMCPTool(tt.name)
		if got != tt.want {
			t.Errorf("IsMCPTool(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseMCPToolName(t *testing.T) {
	tests := []struct {
		input      string
		wantServer string
		wantTool   string
		wantOk     bool
	}{
		{"mcp_filesystem__read_file", "filesystem", "read_file", true},
		{"mcp_web__search", "web", "search", true},
		{"mcp_my-server__my-tool", "my-server", "my-tool", true},
		{"local-skill", "", "", false},
		{"mcp_nodelimiter", "", "", false},
	}

	for _, tt := range tests {
		server, tool, ok := ParseMCPToolName(tt.input)
		if ok != tt.wantOk || server != tt.wantServer || tool != tt.wantTool {
			t.Errorf("ParseMCPToolName(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, server, tool, ok, tt.wantServer, tt.wantTool, tt.wantOk)
		}
	}
}

func TestFormatMCPContent(t *testing.T) {
	contents := []MCPContent{
		{Type: "text", Text: "Hello"},
		{Type: "text", Text: "World"},
	}

	result := formatMCPContent(contents)
	if result != "Hello\nWorld" {
		t.Errorf("expected 'Hello\\nWorld', got %q", result)
	}
}

func TestFormatMCPContentEmpty(t *testing.T) {
	result := formatMCPContent(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestGetAllLLMToolsNilManager(t *testing.T) {
	tools := GetAllLLMTools(nil)
	if len(tools) != 0 {
		t.Errorf("expected 0 tools for nil manager, got %d", len(tools))
	}
}
