package llm

import (
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "provider-a", Provider: "test", Model: "model-a"},
		{Name: "provider-b", Provider: "test", Model: "model-b"},
	}

	if err := r.InitProviders(configs, "provider-a"); err != nil {
		t.Fatalf("InitProviders failed: %v", err)
	}

	// Get 默认
	p, err := r.Default()
	if err != nil {
		t.Fatalf("Default() failed: %v", err)
	}
	if p.Name() != "provider-a" {
		t.Errorf("expected default provider name 'provider-a', got %q", p.Name())
	}

	// Get by name
	p, err = r.Get("provider-b")
	if err != nil {
		t.Fatalf("Get('provider-b') failed: %v", err)
	}
	if p.Name() != "provider-b" {
		t.Errorf("expected provider name 'provider-b', got %q", p.Name())
	}

	// Get non-existent
	_, err = r.Get("non-existent")
	if err == nil {
		t.Error("expected error for non-existent provider")
	}
}

func TestRegistryDefaultName(t *testing.T) {
	r := NewRegistry()

	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "only-one", Provider: "test", Model: "m"},
	}

	// 不指定 defaultName，应自动选择第一个
	if err := r.InitProviders(configs, ""); err != nil {
		t.Fatalf("InitProviders failed: %v", err)
	}

	if r.DefaultName() != "only-one" {
		t.Errorf("expected default name 'only-one', got %q", r.DefaultName())
	}
}

func TestRegistryUnknownProviderType(t *testing.T) {
	r := NewRegistry()
	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "bad", Provider: "unknown-type", Model: "m"},
	}

	err := r.InitProviders(configs, "")
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
}

func TestRegistryInvalidDefault(t *testing.T) {
	r := NewRegistry()
	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "a", Provider: "test", Model: "m"},
	}

	err := r.InitProviders(configs, "non-existent-default")
	if err == nil {
		t.Error("expected error for non-existent default provider")
	}
}

func TestRegistryProviderNames(t *testing.T) {
	r := NewRegistry()
	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "alpha", Provider: "test", Model: "m"},
		{Name: "beta", Provider: "test", Model: "m"},
	}

	if err := r.InitProviders(configs, "alpha"); err != nil {
		t.Fatalf("InitProviders failed: %v", err)
	}

	names := r.ProviderNames()
	if len(names) != 2 {
		t.Errorf("expected 2 provider names, got %d", len(names))
	}
}

func TestRegistryWithPresets(t *testing.T) {
	r := NewRegistry()
	r.RegisterBuiltinFactories()

	configs := []ProviderConfig{
		{Name: "ds", Provider: "deepseek", APIKey: "test-key", Model: "deepseek-chat"},
	}

	if err := r.InitProviders(configs, "ds"); err != nil {
		t.Fatalf("InitProviders with preset failed: %v", err)
	}

	p, err := r.Get("ds")
	if err != nil {
		t.Fatalf("Get('ds') failed: %v", err)
	}
	if p.Name() != "ds" {
		t.Errorf("expected provider name 'ds', got %q", p.Name())
	}
}

func TestRegistryGetEmptyStringUsesDefault(t *testing.T) {
	r := NewRegistry()
	r.Register("test", func(cfg ProviderConfig) (Provider, error) {
		return NewMockProvider(cfg.Name), nil
	})

	configs := []ProviderConfig{
		{Name: "default-p", Provider: "test", Model: "m"},
	}

	if err := r.InitProviders(configs, "default-p"); err != nil {
		t.Fatalf("InitProviders failed: %v", err)
	}

	p, err := r.Get("")
	if err != nil {
		t.Fatalf("Get('') failed: %v", err)
	}
	if p.Name() != "default-p" {
		t.Errorf("expected 'default-p', got %q", p.Name())
	}
}
