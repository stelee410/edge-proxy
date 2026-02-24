package skills

import (
	"context"
	"fmt"
)

// CodeSkill 基于注册 Go handler 的 Skill 实现
// 通过 YAML 配置 handler 名称和参数，运行时调用对应的 CodeHandler
type CodeSkill struct {
	config  SkillConfig
	handler CodeHandler
}

// NewCodeSkill 从配置创建 Code Skill
// globalCfg 传递全局配置（api_key, base_url 等）供 handler 回退使用
func NewCodeSkill(cfg SkillConfig, globalCfg map[string]interface{}) (*CodeSkill, error) {
	if cfg.Handler == "" {
		return nil, fmt.Errorf("code skill %q requires a 'handler' field", cfg.Name)
	}

	handler, err := CreateCodeHandler(cfg.Handler, cfg.Config, globalCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler %q for skill %q: %w", cfg.Handler, cfg.Name, err)
	}

	return &CodeSkill{
		config:  cfg,
		handler: handler,
	}, nil
}

// Name 返回 Skill 名称
func (s *CodeSkill) Name() string { return s.config.Name }

// Stage 返回执行阶段
func (s *CodeSkill) Stage() string { return s.config.Stage }

// Type 返回实现类型
func (s *CodeSkill) Type() string { return TypeCode }

// Definition 返回 Skill 定义
func (s *CodeSkill) Definition() SkillDefinition {
	return s.config.ToDefinition()
}

// Execute 执行 Code Skill：委托给注册的 handler
func (s *CodeSkill) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	return s.handler.Execute(ctx, input)
}
