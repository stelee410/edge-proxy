package llm

import (
	"fmt"
	"sync"
)

// ProviderConfig 单个 Provider 的配置
type ProviderConfig struct {
	Name        string  `yaml:"name"`        // 唯一标识名，如 "openai-gpt4o"
	Provider    string  `yaml:"provider"`    // 提供商类型：openai, ollama, ollama-openai 等
	BaseURL     string  `yaml:"base_url"`    // API 基础地址
	APIKey      string  `yaml:"api_key"`     // API Key
	Model       string  `yaml:"model"`       // 模型名
	Temperature float64 `yaml:"temperature"` // 温度参数
	MaxTokens   int     `yaml:"max_tokens"`  // 最大 token 数
}

// ProviderFactory 创建 Provider 实例的工厂函数
type ProviderFactory func(cfg ProviderConfig) (Provider, error)

// Registry Provider 注册中心，支持动态注册 Provider 工厂函数和多实例管理
type Registry struct {
	mu          sync.RWMutex
	factories   map[string]ProviderFactory // provider type -> factory
	providers   map[string]Provider        // provider name -> instance
	defaultName string                     // 默认 provider 名称
}

// NewRegistry 创建空的 Provider 注册中心
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
		providers: make(map[string]Provider),
	}
}

// Register 注册一个 Provider 工厂函数
// name 为 provider 类型名称（如 "openai", "ollama"），factory 为创建函数
func (r *Registry) Register(name string, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get 根据名称获取已初始化的 Provider 实例
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if name == "" {
		name = r.defaultName
	}
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found (available: %v)", name, r.ProviderNames())
	}
	return p, nil
}

// Default 获取默认 Provider
func (r *Registry) Default() (Provider, error) {
	return r.Get(r.defaultName)
}

// DefaultName 返回默认 Provider 的名称
func (r *Registry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultName
}

// ProviderNames 返回所有已注册 Provider 实例的名称
func (r *Registry) ProviderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// InitProviders 根据配置列表批量初始化 Provider 实例
// 自动应用预设配置（Preset）：如果 provider 字段匹配预设名，会自动填充 base_url 等默认值
func (r *Registry) InitProviders(configs []ProviderConfig, defaultName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cfg := range configs {
		// 应用预设配置：将预设名解析为实际 provider 类型，并填充默认值
		ApplyPreset(&cfg)

		factory, ok := r.factories[cfg.Provider]
		if !ok {
			return fmt.Errorf("unknown provider type %q for %q (registered types: %v, presets: %v)",
				cfg.Provider, cfg.Name, r.factoryNames(), ListPresets())
		}

		provider, err := factory(cfg)
		if err != nil {
			return fmt.Errorf("failed to create provider %q: %w", cfg.Name, err)
		}

		r.providers[cfg.Name] = provider
	}

	if defaultName != "" {
		if _, ok := r.providers[defaultName]; !ok {
			return fmt.Errorf("default provider %q not found in configured providers", defaultName)
		}
		r.defaultName = defaultName
	} else if len(r.providers) > 0 {
		// 未指定 default 时，使用第一个 provider
		for name := range r.providers {
			r.defaultName = name
			break
		}
	}

	return nil
}

// BuildFallbackProvider 基于 fallback 名称列表构建带降级策略的 Provider
// 返回包装后的 FallbackProvider（如果有 fallback 配置），否则返回原始的默认 Provider
func (r *Registry) BuildFallbackProvider(fallbackNames []string) (Provider, error) {
	primary, err := r.Default()
	if err != nil {
		return nil, err
	}

	if len(fallbackNames) == 0 {
		return primary, nil
	}

	fallbacks := make([]Provider, 0, len(fallbackNames))
	for _, name := range fallbackNames {
		p, err := r.Get(name)
		if err != nil {
			return nil, fmt.Errorf("fallback provider %q not found: %w", name, err)
		}
		fallbacks = append(fallbacks, p)
	}

	return NewFallbackProvider(primary, fallbacks), nil
}

// factoryNames 返回已注册的工厂类型名称（内部方法，调用者需持有锁）
func (r *Registry) factoryNames() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// RegisterBuiltinFactories 注册内置的 Provider 工厂
func (r *Registry) RegisterBuiltinFactories() {
	r.Register("openai", func(cfg ProviderConfig) (Provider, error) {
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return NewOpenAIProvider(cfg.Name, baseURL, cfg.APIKey, cfg.Model), nil
	})

	r.Register("ollama", func(cfg ProviderConfig) (Provider, error) {
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllamaProvider(cfg.Name, baseURL, cfg.Model), nil
	})

	r.Register("ollama-openai", func(cfg ProviderConfig) (Provider, error) {
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
		return NewOpenAIProvider(cfg.Name, baseURL, "", cfg.Model), nil
	})

	r.Register("claude", func(cfg ProviderConfig) (Provider, error) {
		return NewClaudeProvider(cfg.Name, cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	})

	r.Register("gemini", func(cfg ProviderConfig) (Provider, error) {
		return NewGeminiProvider(cfg.Name, cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	})
}
