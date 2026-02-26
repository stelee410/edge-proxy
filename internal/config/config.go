package config

import (
	"fmt"
	"os"
	"time"

	"linkyun-edge-proxy/internal/llm"
	"linkyun-edge-proxy/internal/mcp"

	"gopkg.in/yaml.v3"
)

// Config Edge Proxy 配置
type Config struct {
	ServerURL string `yaml:"server_url"` // Linkyun Server 地址
	EdgeToken string `yaml:"edge_token"` // Edge Token
	AgentUUID string `yaml:"agent_uuid"` // Agent UUID

	LLM     LLMConfig     `yaml:"llm"`     // LLM 配置
	Rules   RulesConfig   `yaml:"rules"`   // Rules 配置
	Skills  SkillsConfig  `yaml:"skills"`  // Skills 配置
	MCP     MCPConfig     `yaml:"mcp"`     // MCP 配置
	Sandbox SandboxConfig `yaml:"sandbox"` // Bash 沙箱配置（run_shell 工具）

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // 心跳间隔
	PollTimeout       time.Duration `yaml:"poll_timeout"`       // 轮询超时
	LogLevel          string        `yaml:"log_level"`          // 日志级别：debug, info, warn, error
}

// RulesConfig Rules 文件配置
type RulesConfig struct {
	Enabled     bool     `yaml:"enabled"`     // 是否启用 Rules
	Directories []string `yaml:"directories"` // Rules 文件目录列表
}

// SkillsConfig Skills 配置
type SkillsConfig struct {
	Enabled   bool   `yaml:"enabled"`   // 是否启用 Skills
	Directory string `yaml:"directory"` // Skill 定义文件目录
}

// MCPConfig MCP 配置
type MCPConfig struct {
	Enabled bool               `yaml:"enabled"` // 是否启用 MCP
	Servers []mcp.ServerConfig `yaml:"servers"` // MCP 服务器列表
}

// SandboxConfig Bash 沙箱配置（用于 run_shell 工具）
type SandboxConfig struct {
	Enabled        bool     `yaml:"enabled"`          // 是否启用沙箱（暴露 run_shell 工具）
	WorkDir        string   `yaml:"work_dir"`          // 沙箱工作目录，为空时使用默认
	TimeoutSeconds int      `yaml:"timeout_seconds"`   // 单次执行超时（秒），默认 30
	BashCommand    string   `yaml:"bash_command"`     // bash 可执行路径，如 "bash"、"wsl" 或 Git Bash 路径，为空时用 "bash"
	ExtraBlacklist []string `yaml:"extra_blacklist"`   // 额外黑名单模式（子串匹配），与内置危险命令一起生效
}

// LLMConfig LLM 配置，同时支持新旧两种格式
// 新格式：llm.default + llm.providers[] + llm.fallback[]
// 旧格式：llm.provider + llm.base_url + ... （向后兼容）
type LLMConfig struct {
	// 新格式：多 Provider
	Default   string              `yaml:"default"`   // 默认 Provider 名称
	Fallback  []string            `yaml:"fallback"`  // 降级 Provider 名称列表
	Providers []llm.ProviderConfig `yaml:"providers"` // Provider 配置列表

	// 旧格式：单 Provider（向后兼容）
	Provider    string  `yaml:"provider"`    // 提供商：openai, ollama 等
	BaseURL     string  `yaml:"base_url"`    // API 基础地址
	APIKey      string  `yaml:"api_key"`     // API Key（如需要）
	Model       string  `yaml:"model"`       // 模型名
	Temperature float64 `yaml:"temperature"` // 温度参数
	MaxTokens   int     `yaml:"max_tokens"`  // 最大 token 数
}

// GetProviderConfigs 返回标准化后的 Provider 配置列表和默认名称
// 自动处理旧格式到新格式的转换
func (c *LLMConfig) GetProviderConfigs() ([]llm.ProviderConfig, string) {
	if len(c.Providers) > 0 {
		return c.Providers, c.Default
	}

	// 旧格式兼容：将单个 provider 配置转换为 providers 数组
	if c.Provider != "" {
		name := c.Provider
		if c.Model != "" {
			name = c.Provider + "-" + c.Model
		}
		cfg := llm.ProviderConfig{
			Name:        name,
			Provider:    c.Provider,
			BaseURL:     c.BaseURL,
			APIKey:      c.APIKey,
			Model:       c.Model,
			Temperature: c.Temperature,
			MaxTokens:   c.MaxTokens,
		}
		return []llm.ProviderConfig{cfg}, name
	}

	return nil, ""
}

// Load 从 YAML 文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{
		HeartbeatInterval: 15 * time.Second,
		PollTimeout:       30 * time.Second,
		LogLevel:          "info",
		LLM: LLMConfig{
			Temperature: 0.7,
			MaxTokens:   4096,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if cfg.EdgeToken == "" {
		return nil, fmt.Errorf("edge_token is required")
	}
	if cfg.AgentUUID == "" {
		return nil, fmt.Errorf("agent_uuid is required")
	}

	// 验证 LLM 配置：新格式或旧格式至少有一个
	providerConfigs, _ := cfg.LLM.GetProviderConfigs()
	if len(providerConfigs) == 0 {
		return nil, fmt.Errorf("llm configuration is required: use either llm.providers[] (new) or llm.provider (legacy)")
	}

	// 验证每个 provider 配置
	for i, pc := range providerConfigs {
		if pc.Provider == "" {
			return nil, fmt.Errorf("llm.providers[%d].provider is required", i)
		}
		if pc.Model == "" {
			return nil, fmt.Errorf("llm.providers[%d].model is required", i)
		}
		if pc.Name == "" {
			return nil, fmt.Errorf("llm.providers[%d].name is required", i)
		}
	}

	return cfg, nil
}
