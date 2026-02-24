package llm

import (
	"context"
	"fmt"
)

// ContentPart 多模态内容块（文本或图片）
type ContentPart struct {
	Type        string `json:"type"` // "text" | "image"
	Text        string `json:"text,omitempty"`
	ImageBase64 string `json:"image_base64,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
}

// Message LLM 消息
type Message struct {
	Role         string       `json:"role"`
	Content      string       `json:"content"`
	ContentParts []ContentPart `json:"content_parts,omitempty"` // 多模态：当非空时优先于 Content（图片+文本）
	ToolCalls    []ToolCall   `json:"tool_calls,omitempty"`     // assistant 消息中的 tool calls（用于多轮 tool calling 历史）
}

// CompletionRequest LLM 补全请求
type CompletionRequest struct {
	Model        string           `json:"model"`
	Messages     []Message        `json:"messages"`
	Temperature  float64          `json:"temperature"`
	MaxTokens    int              `json:"max_tokens"`
	SystemPrompt string           `json:"system_prompt,omitempty"`
	Tools        []ToolDefinition `json:"tools,omitempty"`
	ToolResults  []ToolResult     `json:"tool_results,omitempty"` // 上一轮 tool call 的结果
}

// CompletionResponse LLM 补全响应
type CompletionResponse struct {
	Content      string     `json:"content"`
	Model        string     `json:"model"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	TotalTokens  int        `json:"total_tokens"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"` // LLM 请求调用的工具
	StopReason   string     `json:"stop_reason,omitempty"`
}

// HasToolCalls 判断响应是否包含 tool calls
func (r *CompletionResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// ToolDefinition 工具定义，用于传递给 LLM
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"` // JSON Schema
}

// ToolCall LLM 返回的工具调用请求
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// StreamEvent 流式响应事件
type StreamEvent struct {
	Content string // 增量文本内容
	Done    bool   // 是否结束
	Error   error  // 错误信息
}

// Provider LLM 提供商接口
type Provider interface {
	// Name 返回 Provider 实例的名称标识
	Name() string
	// Complete 同步补全请求
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	// StreamComplete 流式补全请求（暂未实现，预留接口）
	StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error)
}

// NewProvider 根据配置创建 LLM Provider（向后兼容的便捷方法）
func NewProvider(provider, baseURL, apiKey, model string) (Provider, error) {
	switch provider {
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return NewOpenAIProvider(provider, baseURL, apiKey, model), nil
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllamaProvider(provider, baseURL, model), nil
	case "ollama-openai":
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
		return NewOpenAIProvider(provider, baseURL, "", model), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, ollama, ollama-openai)", provider)
	}
}
