package llm

import (
	"context"
	"fmt"
	"testing"
)

func TestFallbackProviderPrimarySuccess(t *testing.T) {
	primary := NewMockProvider("primary")
	fb1 := NewMockProvider("fallback-1")

	fp := NewFallbackProvider(primary, []Provider{fb1})

	resp, err := fp.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response from primary" {
		t.Errorf("expected response from primary, got %q", resp.Content)
	}
	if primary.CallCount() != 1 {
		t.Errorf("expected primary called once, got %d", primary.CallCount())
	}
	if fb1.CallCount() != 0 {
		t.Errorf("expected fallback not called, got %d", fb1.CallCount())
	}
}

func TestFallbackProviderPrimaryFails(t *testing.T) {
	primary := NewMockProviderWithError("primary", fmt.Errorf("primary error"))
	fb1 := NewMockProvider("fallback-1")

	fp := NewFallbackProvider(primary, []Provider{fb1})

	resp, err := fp.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response from fallback-1" {
		t.Errorf("expected response from fallback-1, got %q", resp.Content)
	}
	if fb1.CallCount() != 1 {
		t.Errorf("expected fallback called once, got %d", fb1.CallCount())
	}
}

func TestFallbackProviderAllFail(t *testing.T) {
	primary := NewMockProviderWithError("primary", fmt.Errorf("primary error"))
	fb1 := NewMockProviderWithError("fallback-1", fmt.Errorf("fallback-1 error"))
	fb2 := NewMockProviderWithError("fallback-2", fmt.Errorf("fallback-2 error"))

	fp := NewFallbackProvider(primary, []Provider{fb1, fb2})

	_, err := fp.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	// 错误信息应包含 primary 的错误
	if primary.CallCount() != 1 || fb1.CallCount() != 1 || fb2.CallCount() != 1 {
		t.Errorf("expected all providers called once: primary=%d, fb1=%d, fb2=%d",
			primary.CallCount(), fb1.CallCount(), fb2.CallCount())
	}
}

func TestFallbackProviderNoFallbacks(t *testing.T) {
	primary := NewMockProviderWithError("primary", fmt.Errorf("primary error"))

	fp := NewFallbackProvider(primary, nil)

	_, err := fp.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error when primary fails with no fallbacks")
	}
}

func TestFallbackProviderName(t *testing.T) {
	primary := NewMockProvider("my-primary")
	fp := NewFallbackProvider(primary, nil)

	if fp.Name() != "my-primary" {
		t.Errorf("expected name 'my-primary', got %q", fp.Name())
	}
}

func TestFallbackProviderSecondFallbackSucceeds(t *testing.T) {
	primary := NewMockProviderWithError("primary", fmt.Errorf("primary error"))
	fb1 := NewMockProviderWithError("fallback-1", fmt.Errorf("fallback-1 error"))
	fb2 := NewMockProvider("fallback-2")

	fp := NewFallbackProvider(primary, []Provider{fb1, fb2})

	resp, err := fp.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response from fallback-2" {
		t.Errorf("expected response from fallback-2, got %q", resp.Content)
	}
	if primary.CallCount() != 1 || fb1.CallCount() != 1 || fb2.CallCount() != 1 {
		t.Errorf("expected primary and fb1 called once, fb2 called once")
	}
}

func TestBuildFallbackProvider(t *testing.T) {
	r := NewRegistry()
	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "primary", Provider: "test", Model: "m"},
		{Name: "backup1", Provider: "test", Model: "m"},
		{Name: "backup2", Provider: "test", Model: "m"},
	}

	if err := r.InitProviders(configs, "primary"); err != nil {
		t.Fatalf("InitProviders failed: %v", err)
	}

	// 带 fallback
	p, err := r.BuildFallbackProvider([]string{"backup1", "backup2"})
	if err != nil {
		t.Fatalf("BuildFallbackProvider failed: %v", err)
	}
	if p.Name() != "primary" {
		t.Errorf("expected name 'primary', got %q", p.Name())
	}

	// 无 fallback
	p, err = r.BuildFallbackProvider(nil)
	if err != nil {
		t.Fatalf("BuildFallbackProvider(nil) failed: %v", err)
	}
	if p.Name() != "primary" {
		t.Errorf("expected name 'primary', got %q", p.Name())
	}

	// 无效 fallback
	_, err = r.BuildFallbackProvider([]string{"non-existent"})
	if err == nil {
		t.Error("expected error for non-existent fallback")
	}
}
