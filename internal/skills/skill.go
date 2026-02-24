package skills

import (
	"context"
	"encoding/json"
)

// Skill 阶段常量
const (
	StagePreConversation  = "pre_conversation"
	StageMidConversation  = "mid_conversation"
	StagePostConversation = "post_conversation"
)

// Skill 类型常量
const (
	TypePromptBased = "prompt-based"
	TypePromptAPI   = "prompt-api"
	TypeCode        = "code" // 可执行代码的 Skill，由注册的 Go handler 处理
)

// Skill 本地 Skill 接口
type Skill interface {
	// Name 返回 Skill 名称
	Name() string
	// Stage 返回执行阶段
	Stage() string
	// Type 返回实现类型
	Type() string
	// Execute 执行 Skill
	Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error)
	// Definition 返回 Skill 定义（用于 LLM tool calling）
	Definition() SkillDefinition
}

// SkillInput Skill 输入
type SkillInput struct {
	Arguments   map[string]interface{} `json:"arguments"`
	UserMessage string                 `json:"user_message,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// SkillOutput Skill 输出
type SkillOutput struct {
	Content  string                 `json:"content"`
	Success  bool                   `json:"success"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// SkillDefinition Skill 定义，与 Claude/OpenAI 的 tool definition 格式兼容
type SkillDefinition struct {
	Name            string          `json:"name" yaml:"name"`
	Description     string          `json:"description" yaml:"description"`
	DescriptionLLM  string          `json:"description_for_llm,omitempty" yaml:"description_for_llm"`
	Stage           string          `json:"stage" yaml:"stage"`
	Type            string          `json:"type" yaml:"type"`
	InputSchema     json.RawMessage `json:"input_schema,omitempty" yaml:"input_schema"`
	Enabled         bool            `json:"enabled" yaml:"enabled"`
}

// SkillConfig 从 YAML 文件加载的 Skill 配置
type SkillConfig struct {
	Name            string                 `yaml:"name"`
	Description     string                 `yaml:"description"`
	DescriptionLLM  string                 `yaml:"description_for_llm"`
	Stage           string                 `yaml:"stage"`
	Type            string                 `yaml:"type"`
	Enabled         *bool                  `yaml:"enabled"`
	InputSchema     map[string]interface{} `yaml:"input_schema"`

	// prompt-based 相关
	PromptTemplate  string                 `yaml:"prompt_template"`

	// prompt-api 相关
	APIURL          string                 `yaml:"api_url"`
	APIMethod       string                 `yaml:"api_method"`
	APIHeaders      map[string]string      `yaml:"api_headers"`

	// code 类型相关
	Handler string                 `yaml:"handler"` // handler 名称，映射到注册的 Go handler
	Config  map[string]interface{} `yaml:"config"`  // handler 配置参数
}

// IsEnabled 返回 Skill 是否启用，默认为 true
func (c *SkillConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}
