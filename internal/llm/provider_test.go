package llm

import (
	"context"
	"testing"
)

func TestMockProviderComplete(t *testing.T) {
	p := NewMockProvider("test")

	resp, err := p.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response from test" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if p.CallCount() != 1 {
		t.Errorf("expected call count 1, got %d", p.CallCount())
	}
}

func TestMockProviderWithError(t *testing.T) {
	p := NewMockProviderWithError("test", context.DeadlineExceeded)

	_, err := p.Complete(context.Background(), &CompletionRequest{})
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestMockProviderWithFunc(t *testing.T) {
	callCount := 0
	p := NewMockProviderWithFunc("custom", func(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
		callCount++
		return &CompletionResponse{Content: "custom-" + req.Messages[0].Content}, nil
	})

	resp, err := p.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "custom-hello" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if callCount != 1 {
		t.Errorf("expected callCount 1, got %d", callCount)
	}
}

func TestMockProviderName(t *testing.T) {
	p := NewMockProvider("my-name")
	if p.Name() != "my-name" {
		t.Errorf("expected name 'my-name', got %q", p.Name())
	}
}

func TestMockProviderStreamComplete(t *testing.T) {
	p := NewMockProvider("test")
	_, err := p.StreamComplete(context.Background(), &CompletionRequest{})
	if err == nil {
		t.Error("expected error from StreamComplete")
	}
}

func TestPresetApply(t *testing.T) {
	// Test deepseek preset
	cfg := ProviderConfig{
		Name:     "ds",
		Provider: "deepseek",
		APIKey:   "key",
		Model:    "deepseek-chat",
	}
	ApplyPreset(&cfg)

	if cfg.Provider != "openai" {
		t.Errorf("expected provider 'openai' after preset, got %q", cfg.Provider)
	}
	if cfg.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("expected deepseek base URL, got %q", cfg.BaseURL)
	}
}

func TestPresetApplyDoesNotOverrideExplicitBaseURL(t *testing.T) {
	cfg := ProviderConfig{
		Name:     "custom-ds",
		Provider: "deepseek",
		BaseURL:  "https://custom.endpoint.com/v1",
		APIKey:   "key",
		Model:    "deepseek-chat",
	}
	ApplyPreset(&cfg)

	if cfg.BaseURL != "https://custom.endpoint.com/v1" {
		t.Errorf("expected custom base URL preserved, got %q", cfg.BaseURL)
	}
}

func TestPresetApplyUnknownProvider(t *testing.T) {
	cfg := ProviderConfig{
		Name:     "custom",
		Provider: "some-unknown-provider",
		BaseURL:  "https://example.com",
		Model:    "m",
	}
	originalProvider := cfg.Provider
	ApplyPreset(&cfg)

	// 未知预设不应改变 provider
	if cfg.Provider != originalProvider {
		t.Errorf("expected provider unchanged for unknown preset, got %q", cfg.Provider)
	}
}

func TestListPresets(t *testing.T) {
	presets := ListPresets()
	if len(presets) == 0 {
		t.Error("expected non-empty preset list")
	}

	// 检查关键预设是否存在
	expected := []string{"openai", "claude", "gemini", "deepseek", "qwen", "doubao", "moonshot", "zhipu"}
	presetSet := make(map[string]bool)
	for _, p := range presets {
		presetSet[p] = true
	}
	for _, name := range expected {
		if !presetSet[name] {
			t.Errorf("expected preset %q not found", name)
		}
	}
}
