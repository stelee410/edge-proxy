package rules

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Engine 规则引擎，管理已加载的规则并提供上下文注入能力
type Engine struct {
	mu          sync.RWMutex
	ruleSet     *RuleSet
	loader      *Loader
	directories []string
}

// NewEngine 创建规则引擎
func NewEngine(directories []string) *Engine {
	return &Engine{
		loader:      NewLoader(directories),
		ruleSet:     NewRuleSet(),
		directories: directories,
	}
}

// StartWatching 启动文件监听，在 rules 文件变化时自动重载
func (e *Engine) StartWatching(ctx context.Context) error {
	watcher := NewWatcher(e, e.directories)
	return watcher.Start(ctx)
}

// LoadRules 加载（或重新加载）所有规则
func (e *Engine) LoadRules() error {
	ruleSet, err := e.loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.ruleSet = ruleSet

	return nil
}

// GetApplicableRules 获取当前适用的规则
// 当前实现：返回所有 alwaysApply=true 的规则
// 后续可扩展：根据 globs、会话类型等进行条件筛选
func (e *Engine) GetApplicableRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ruleSet.GetAlwaysApply()
}

// GetAllRules 返回所有已加载的规则
func (e *Engine) GetAllRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ruleSet.Rules
}

// RuleCount 返回已加载的规则总数
func (e *Engine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ruleSet.Count()
}

// BuildContext 将适用的规则合并为注入到 system prompt 的上下文文本
// 返回空字符串表示没有可注入的规则
func (e *Engine) BuildContext() string {
	rules := e.GetApplicableRules()
	if len(rules) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n--- Rules ---\n")

	for i, r := range rules {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[Rule %d: %s]\n", i+1, r.Name))
		sb.WriteString(r.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}

// InjectIntoSystemPrompt 将规则上下文注入到系统提示词中
// 规则内容追加到原始 system prompt 之后，不覆盖原始内容
func (e *Engine) InjectIntoSystemPrompt(originalPrompt string) string {
	ctx := e.BuildContext()
	if ctx == "" {
		return originalPrompt
	}
	return originalPrompt + ctx
}
