package skills

import (
	"context"
	"encoding/json"
	"fmt"
)

// PromptBasedSkill 基于模板的 Skill 实现
// 通过 Go text/template 渲染模板，将结果作为 tool result 或 prompt 补充返回
type PromptBasedSkill struct {
	config SkillConfig
}

// NewPromptBasedSkill 从配置创建 Prompt-based Skill
func NewPromptBasedSkill(cfg SkillConfig) *PromptBasedSkill {
	return &PromptBasedSkill{config: cfg}
}

// Name 返回 Skill 名称
func (s *PromptBasedSkill) Name() string { return s.config.Name }

// Stage 返回执行阶段
func (s *PromptBasedSkill) Stage() string { return s.config.Stage }

// Type 返回实现类型
func (s *PromptBasedSkill) Type() string { return TypePromptBased }

// Definition 返回 Skill 定义
func (s *PromptBasedSkill) Definition() SkillDefinition {
	return s.config.ToDefinition()
}

// Execute 执行 Prompt-based Skill：渲染模板并返回结果
func (s *PromptBasedSkill) Execute(_ context.Context, input *SkillInput) (*SkillOutput, error) {
	if s.config.PromptTemplate == "" {
		return &SkillOutput{
			Success: false,
			Error:   "prompt_template is not configured",
		}, fmt.Errorf("prompt_template is not configured for skill %q", s.config.Name)
	}

	// 准备模板数据
	data := make(map[string]interface{})
	if input != nil && input.Arguments != nil {
		for k, v := range input.Arguments {
			data[k] = v
		}
	}
	if input != nil && input.UserMessage != "" {
		data["user_message"] = input.UserMessage
	}
	if input != nil && input.Context != nil {
		for k, v := range input.Context {
			data[k] = v
		}
	}

	// 渲染模板
	result, err := RenderTemplate(s.config.PromptTemplate, data)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("template rendering failed: %v", err),
		}, err
	}

	return &SkillOutput{
		Content: result,
		Success: true,
		Metadata: map[string]interface{}{
			"skill_name": s.config.Name,
			"skill_type": TypePromptBased,
		},
	}, nil
}

// NewSkillFromConfig 根据配置创建对应类型的 Skill
// globalCfg 为全局配置（如 llm_api_key, llm_base_url），供 code handler 做 fallback
func NewSkillFromConfig(cfg SkillConfig, globalCfg map[string]interface{}) (Skill, error) {
	switch cfg.Type {
	case TypePromptBased:
		return NewPromptBasedSkill(cfg), nil
	case TypePromptAPI:
		return NewPromptAPISkill(cfg), nil
	case TypeCode:
		return NewCodeSkill(cfg, globalCfg)
	default:
		return nil, fmt.Errorf("unknown skill type %q for skill %q", cfg.Type, cfg.Name)
	}
}

// LoadAndRegisterSkills 从目录加载 Skills 并注册到 Registry
// globalCfg 为全局配置，传递给 code 类型 Skill 的 handler
func LoadAndRegisterSkills(dir string, registry *Registry, globalCfg map[string]interface{}) (int, error) {
	configs, err := LoadSkillsFromDirectory(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to load skills: %w", err)
	}

	count := 0
	for _, cfg := range configs {
		skill, err := NewSkillFromConfig(*cfg, globalCfg)
		if err != nil {
			return count, fmt.Errorf("failed to create skill %q: %w", cfg.Name, err)
		}

		if err := registry.Register(skill); err != nil {
			return count, fmt.Errorf("failed to register skill %q: %w", cfg.Name, err)
		}
		count++
	}

	// 将 InputSchema 序列化为 json.RawMessage 用于 Definition
	_ = json.Marshal
	return count, nil
}
