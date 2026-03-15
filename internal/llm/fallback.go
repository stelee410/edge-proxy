package llm

import (
	"context"
	"fmt"
)

// FallbackProvider 带降级策略的 Provider 包装器
// 当主 Provider 调用失败时，自动依次尝试 fallback 列表中的 Provider
type FallbackProvider struct {
	primary   Provider
	fallbacks []Provider
}

// NewFallbackProvider 创建带 Fallback 的 Provider
// primary 为主 Provider，fallbacks 为按优先级排列的备用 Provider 列表
func NewFallbackProvider(primary Provider, fallbacks []Provider) *FallbackProvider {
	return &FallbackProvider{
		primary:   primary,
		fallbacks: fallbacks,
	}
}

// Name 返回主 Provider 名称
func (p *FallbackProvider) Name() string {
	return p.primary.Name()
}

// Complete 先尝试主 Provider，失败后依次尝试 fallback
func (p *FallbackProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	resp, err := p.primary.Complete(ctx, req)
	if err == nil {
		return resp, nil
	}

	primaryErr := err
	for _, fb := range p.fallbacks {
		resp, err = fb.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
	}

	// 所有 Provider 都失败，返回主 Provider 的错误
	if len(p.fallbacks) > 0 {
		return nil, fmt.Errorf("all providers failed, primary (%s): %w", p.primary.Name(), primaryErr)
	}
	return nil, primaryErr
}

// StreamComplete 流式补全 - 先尝试主 Provider，失败后依次尝试 fallback
func (p *FallbackProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	ch, err := p.primary.StreamComplete(ctx, req)
	if err == nil {
		return ch, nil
	}

	primaryErr := err
	for _, fb := range p.fallbacks {
		ch, err = fb.StreamComplete(ctx, req)
		if err == nil {
			return ch, nil
		}
	}

	if len(p.fallbacks) > 0 {
		return nil, fmt.Errorf("all providers StreamComplete failed, primary (%s): %w", p.primary.Name(), primaryErr)
	}
	return nil, primaryErr
}
