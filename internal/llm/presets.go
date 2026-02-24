package llm

// Preset 预设配置，为已知的 LLM 提供商预填充默认值
type Preset struct {
	ProviderType string // 实际使用的 Provider 工厂类型（如 "openai", "claude", "gemini"）
	BaseURL      string // 预设的 API 基础地址
	Description  string // 描述信息
}

// builtinPresets 内置预设列表
// key 为预设名称（即用户配置中的 provider 字段值）
var builtinPresets = map[string]Preset{
	// --- 国际模型 ---
	"openai": {
		ProviderType: "openai",
		BaseURL:      "https://api.openai.com/v1",
		Description:  "OpenAI GPT 系列",
	},
	"claude": {
		ProviderType: "claude",
		BaseURL:      "https://api.anthropic.com",
		Description:  "Anthropic Claude 系列",
	},
	"gemini": {
		ProviderType: "gemini",
		BaseURL:      "https://generativelanguage.googleapis.com",
		Description:  "Google Gemini 系列",
	},
	"ollama": {
		ProviderType: "ollama",
		BaseURL:      "http://localhost:11434",
		Description:  "Ollama 本地模型（原生 API）",
	},
	"ollama-openai": {
		ProviderType: "ollama-openai",
		BaseURL:      "http://localhost:11434/v1",
		Description:  "Ollama 本地模型（OpenAI 兼容模式）",
	},

	// --- 国产模型（均兼容 OpenAI API） ---
	"deepseek": {
		ProviderType: "openai",
		BaseURL:      "https://api.deepseek.com/v1",
		Description:  "DeepSeek 深度求索",
	},
	"qwen": {
		ProviderType: "openai",
		BaseURL:      "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Description:  "通义千问 Qwen（阿里云百炼）",
	},
	"doubao": {
		ProviderType: "openai",
		BaseURL:      "https://ark.cn-beijing.volces.com/api/v3",
		Description:  "豆包 Doubao（火山引擎）",
	},
	"moonshot": {
		ProviderType: "openai",
		BaseURL:      "https://api.moonshot.cn/v1",
		Description:  "Moonshot / Kimi",
	},
	"zhipu": {
		ProviderType: "openai",
		BaseURL:      "https://open.bigmodel.cn/api/paas/v4",
		Description:  "智谱 GLM（智谱 AI）",
	},
	"ernie": {
		ProviderType: "openai",
		BaseURL:      "https://qianfan.baidubce.com/v2",
		Description:  "百度文心 ERNIE（千帆平台 OpenAI 兼容端点）",
	},
}

// GetPreset 查询预设配置，找不到则返回 nil
func GetPreset(name string) *Preset {
	p, ok := builtinPresets[name]
	if !ok {
		return nil
	}
	return &p
}

// ListPresets 返回所有可用的预设名称
func ListPresets() []string {
	names := make([]string, 0, len(builtinPresets))
	for name := range builtinPresets {
		names = append(names, name)
	}
	return names
}

// ApplyPreset 将预设配置应用到 ProviderConfig
// 仅填充用户未显式设置的字段
func ApplyPreset(cfg *ProviderConfig) {
	preset := GetPreset(cfg.Provider)
	if preset == nil {
		return
	}

	// 将 provider 替换为实际的工厂类型
	cfg.Provider = preset.ProviderType

	// 仅在用户未设置 base_url 时使用预设值
	if cfg.BaseURL == "" {
		cfg.BaseURL = preset.BaseURL
	}
}
