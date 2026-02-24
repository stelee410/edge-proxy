package llm

import (
	"context"
	"fmt"
)

// MockProvider 用于测试的 Mock LLM Provider
type MockProvider struct {
	name         string
	completeFunc func(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	callCount    int
}

// NewMockProvider 创建 Mock Provider
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name: name,
		completeFunc: func(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
			return &CompletionResponse{
				Content:     "mock response from " + name,
				Model:       "mock-model",
				TotalTokens: 10,
			}, nil
		},
	}
}

// NewMockProviderWithError 创建始终返回错误的 Mock Provider
func NewMockProviderWithError(name string, err error) *MockProvider {
	return &MockProvider{
		name: name,
		completeFunc: func(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
			return nil, err
		},
	}
}

// NewMockProviderWithFunc 创建自定义处理函数的 Mock Provider
func NewMockProviderWithFunc(name string, fn func(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)) *MockProvider {
	return &MockProvider{
		name:         name,
		completeFunc: fn,
	}
}

// Name 返回 Provider 名称
func (p *MockProvider) Name() string {
	return p.name
}

// Complete 调用 mock 函数
func (p *MockProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	p.callCount++
	return p.completeFunc(ctx, req)
}

// StreamComplete 流式补全（暂未实现）
func (p *MockProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("StreamComplete not implemented for mock provider %q", p.name)
}

// CallCount 返回 Complete 被调用的次数
func (p *MockProvider) CallCount() int {
	return p.callCount
}

// ResetCallCount 重置调用计数
func (p *MockProvider) ResetCallCount() {
	p.callCount = 0
}
