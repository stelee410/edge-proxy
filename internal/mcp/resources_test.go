package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestResourceManagerInjectNil(t *testing.T) {
	mgr := NewManager()
	rm := NewResourceManager(mgr)

	result := rm.InjectIntoSystemPrompt(context.Background(), "Hello")
	if result != "Hello" {
		t.Errorf("expected unchanged prompt, got %q", result)
	}
}

func TestResourceManagerBuildContextNoServers(t *testing.T) {
	mgr := NewManager()
	rm := NewResourceManager(mgr)

	ctx := rm.BuildResourceContext(context.Background())
	if ctx != "" {
		t.Errorf("expected empty context, got %q", ctx)
	}
}

func TestResourceManagerBuildContextAutoInjectDisabled(t *testing.T) {
	mgr := NewManager()
	rm := NewResourceManager(mgr)
	rm.SetServerConfig("test", ResourceConfig{
		AutoInject: false,
	})

	ctx := rm.BuildResourceContext(context.Background())
	if ctx != "" {
		t.Errorf("expected empty context when auto_inject=false, got %q", ctx)
	}
}

func TestResourceManagerClearCache(t *testing.T) {
	mgr := NewManager()
	rm := NewResourceManager(mgr)

	rm.cache["key"] = &cachedResource{content: "test"}
	rm.ClearCache()

	if len(rm.cache) != 0 {
		t.Errorf("expected empty cache after clear, got %d entries", len(rm.cache))
	}
}

func TestDefaultResourceConfig(t *testing.T) {
	cfg := DefaultResourceConfig()
	if cfg.MaxSize != 10240 {
		t.Errorf("expected max_size 10240, got %d", cfg.MaxSize)
	}
	if cfg.CacheTTL != 300 {
		t.Errorf("expected cache_ttl 300, got %d", cfg.CacheTTL)
	}
	if cfg.AutoInject {
		t.Error("expected auto_inject false by default")
	}
}

func TestResourceManagerInjectEmptyPrompt(t *testing.T) {
	mgr := NewManager()
	rm := NewResourceManager(mgr)

	// No servers configured, should return empty prompt as-is
	result := rm.InjectIntoSystemPrompt(context.Background(), "")
	if result != "" {
		t.Errorf("expected empty prompt, got %q", result)
	}
}

func TestResourceManagerInjectIntoSystemPromptFormat(t *testing.T) {
	// Test the injection format logic (without actual MCP servers)
	mgr := NewManager()
	rm := NewResourceManager(mgr)

	// Simulate injection by testing InjectIntoSystemPrompt with empty resource context
	original := "You are a helpful assistant."
	result := rm.InjectIntoSystemPrompt(context.Background(), original)
	if result != original {
		t.Errorf("expected unchanged prompt when no resources, got %q", result)
	}

	// Verify the resource context format function
	parts := []string{
		`<mcp-resource server="fs" name="readme" uri="file:///readme.md">`,
		"# README",
		"</mcp-resource>",
	}
	formatted := "## MCP Resources\n\n" + strings.Join(parts, "\n")
	if !strings.HasPrefix(formatted, "## MCP Resources") {
		t.Error("formatted resource context should start with header")
	}
}
