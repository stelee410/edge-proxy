package skills

import (
	"context"
	"sort"
	"sync"

	"linkyun-edge-proxy/internal/logger"
)

// PreResult pre_conversation 阶段的执行结果
type PreResult struct {
	// ExtraSystemPrompt 需要追加到 system prompt 的内容
	ExtraSystemPrompt string
	// ExtraContext 传递给后续阶段的上下文
	ExtraContext map[string]interface{}
}

// PostResult post_conversation 阶段的执行结果
type PostResult struct {
	// Content 处理后的最终内容
	Content string
	// Metadata 各 post skill 的聚合 metadata（如 audio_base64, audio_format 等）
	Metadata map[string]interface{}
}

// Pipeline Skill 执行管道，编排三阶段 Skill 执行
type Pipeline struct {
	registry *Registry
	mu       sync.RWMutex
}

// NewPipeline 创建 Skill 管道
func NewPipeline(registry *Registry) *Pipeline {
	return &Pipeline{
		registry: registry,
	}
}

// ExecutePreConversation 执行 pre_conversation 阶段的所有 Skills
// 按 execution_order 排序执行，每个 Skill 的输出会追加到 ExtraSystemPrompt
// 任一 Skill 执行失败会被跳过，不影响后续 Skill
func (p *Pipeline) ExecutePreConversation(ctx context.Context, input *SkillInput) (*PreResult, error) {
	p.mu.RLock()
	preSkills := p.registry.GetByStage(StagePreConversation)
	p.mu.RUnlock()

	if len(preSkills) == 0 {
		return &PreResult{}, nil
	}

	// 按名称排序，确保执行顺序一致
	sort.Slice(preSkills, func(i, j int) bool {
		return preSkills[i].Name() < preSkills[j].Name()
	})

	result := &PreResult{
		ExtraContext: make(map[string]interface{}),
	}

	for _, skill := range preSkills {
		output, err := skill.Execute(ctx, input)
		if err != nil {
			logger.Warn("Pre-conversation skill %q failed: %v (skipping)", skill.Name(), err)
			continue
		}
		if !output.Success {
			logger.Warn("Pre-conversation skill %q returned error: %s (skipping)", skill.Name(), output.Error)
			continue
		}
		if output.Content != "" {
			if result.ExtraSystemPrompt != "" {
				result.ExtraSystemPrompt += "\n\n"
			}
			result.ExtraSystemPrompt += output.Content
		}
		if output.Metadata != nil {
			for k, v := range output.Metadata {
				result.ExtraContext[k] = v
			}
		}
	}

	return result, nil
}

// GetMidConversationDefinitions 获取 mid_conversation 阶段的 Skill 定义
// 用于传递给 LLM 的 tools 参数
func (p *Pipeline) GetMidConversationDefinitions() []SkillDefinition {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.registry.DefinitionsByStage(StageMidConversation)
}

// ExecutePostConversation 执行 post_conversation 阶段的所有 Skills
// 链式执行：前一个 Skill 的输出作为后一个 Skill 的输入内容
// 各 Skill 的 Metadata 会被聚合到 PostResult 中（后者覆盖前者的同名 key）
// 任一 Skill 执行失败会被跳过，保留上一次成功的内容
func (p *Pipeline) ExecutePostConversation(ctx context.Context, llmOutput string, extraContext map[string]interface{}) (*PostResult, error) {
	result := &PostResult{
		Content:  llmOutput,
		Metadata: make(map[string]interface{}),
	}

	p.mu.RLock()
	postSkills := p.registry.GetByStage(StagePostConversation)
	p.mu.RUnlock()

	if len(postSkills) == 0 {
		return result, nil
	}

	// 按名称排序
	sort.Slice(postSkills, func(i, j int) bool {
		return postSkills[i].Name() < postSkills[j].Name()
	})

	for _, skill := range postSkills {
		input := &SkillInput{
			Arguments: map[string]interface{}{
				"content": result.Content,
			},
			Context: extraContext,
		}

		output, err := skill.Execute(ctx, input)
		if err != nil {
			logger.Warn("Post-conversation skill %q failed: %v (skipping)", skill.Name(), err)
			continue
		}
		if !output.Success {
			logger.Warn("Post-conversation skill %q returned error: %s (skipping)", skill.Name(), output.Error)
			continue
		}
		if output.Content != "" {
			result.Content = output.Content
		}
		// 聚合 metadata
		for k, v := range output.Metadata {
			result.Metadata[k] = v
		}
	}

	return result, nil
}

// Reload 重新加载 Skills（用于热加载）
// globalCfg 传递给 code 类型 handler 使用
func (p *Pipeline) Reload(dir string, globalCfg map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.registry.Clear()
	_, err := LoadAndRegisterSkills(dir, p.registry, globalCfg)
	return err
}

// Registry 获取底层 Registry
func (p *Pipeline) GetRegistry() *Registry {
	return p.registry
}
