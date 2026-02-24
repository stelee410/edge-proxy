package mcp

import (
	"testing"
)

func TestSplitToolName(t *testing.T) {
	tests := []struct {
		input      string
		wantServer string
		wantTool   string
	}{
		{"server__tool", "server", "tool"},
		{"my-server__my-tool", "my-server", "my-tool"},
		{"fs__read_file", "fs", "read_file"},
		{"no-separator", "", "no-separator"},
		{"a__b__c", "a", "b__c"},
	}

	for _, tt := range tests {
		server, tool := splitToolName(tt.input)
		if server != tt.wantServer || tool != tt.wantTool {
			t.Errorf("splitToolName(%q) = (%q, %q), want (%q, %q)",
				tt.input, server, tool, tt.wantServer, tt.wantTool)
		}
	}
}

func TestManagerNewAndEmpty(t *testing.T) {
	mgr := NewManager()
	if mgr.ServerCount() != 0 {
		t.Errorf("expected 0 servers, got %d", mgr.ServerCount())
	}

	tools := mgr.GetAllTools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}

	names := mgr.ServerNames()
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

func TestManagerGetServerNotFound(t *testing.T) {
	mgr := NewManager()
	s := mgr.GetServer("nonexistent")
	if s != nil {
		t.Error("expected nil for nonexistent server")
	}
}

func TestManagerShutdownEmpty(t *testing.T) {
	mgr := NewManager()
	err := mgr.Shutdown()
	if err != nil {
		t.Errorf("Shutdown on empty manager should not error: %v", err)
	}
}

func TestServerInstanceStatus(t *testing.T) {
	instance := NewServerInstance(ServerConfig{
		Name:      "test",
		Transport: "stdio",
		Command:   "echo",
	})

	if instance.Status() != StatusDisconnected {
		t.Errorf("expected status disconnected, got %s", instance.Status())
	}
	if instance.Name() != "test" {
		t.Errorf("expected name 'test', got %q", instance.Name())
	}
}

func TestServerInstanceClose(t *testing.T) {
	instance := NewServerInstance(ServerConfig{
		Name: "test",
	})

	err := instance.Close()
	if err != nil {
		t.Errorf("Close should not error on unstarted instance: %v", err)
	}
	if instance.Status() != StatusDisconnected {
		t.Errorf("expected disconnected after close, got %s", instance.Status())
	}
}
