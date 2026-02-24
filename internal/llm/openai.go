package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIProvider 兼容 OpenAI API 的 LLM Provider（支持 OpenAI, vLLM, LocalAI, Ollama OpenAI-compatible 等）
type OpenAIProvider struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIProvider 创建 OpenAI-compatible Provider
func NewOpenAIProvider(name, baseURL, apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回 Provider 名称
func (p *OpenAIProvider) Name() string {
	return p.name
}

// StreamComplete 流式补全（暂未实现）
func (p *OpenAIProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("StreamComplete not implemented for OpenAI provider %q", p.name)
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []openaiTool    `json:"tools,omitempty"`
}

type openaiMessage struct {
	Role       string               `json:"role"`
	Content    interface{}          `json:"content"` // string 或 nil（tool_calls 时可为 nil）
	ToolCalls  []openaiToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content   *string          `json:"content"`
			ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete 实现 Provider 接口
func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	messages := make([]openaiMessage, 0, len(req.Messages)+1+len(req.ToolResults))

	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		msg := openaiMessage{Role: m.Role, Content: m.Content}
		// 多模态：ContentParts 非空时构建 content 数组（图片+文本）
		if len(m.ContentParts) > 0 {
			contentArr := make([]map[string]interface{}, 0, len(m.ContentParts))
			for _, p := range m.ContentParts {
				if p.Type == "text" {
					contentArr = append(contentArr, map[string]interface{}{"type": "text", "text": p.Text})
				} else if p.Type == "image" && p.ImageBase64 != "" {
					mime := p.MimeType
					if mime == "" {
						mime = "image/jpeg"
					}
					contentArr = append(contentArr, map[string]interface{}{
						"type": "image_url",
						"image_url": map[string]string{
							"url": fmt.Sprintf("data:%s;base64,%s", mime, p.ImageBase64),
						},
					})
				}
			}
			msg.Content = contentArr
		}
		// assistant 消息可能带有 tool_calls，需要转换为 OpenAI 格式
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]openaiToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Arguments)
				msg.ToolCalls = append(msg.ToolCalls, openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
			// OpenAI 的 assistant 消息有 tool_calls 时，content 可以为 null
			if m.Content == "" {
				msg.Content = nil
			}
		}
		messages = append(messages, msg)
	}

	// 添加 tool results（作为 tool 角色消息）
	for _, tr := range req.ToolResults {
		messages = append(messages, openaiMessage{
			Role:       "tool",
			Content:    tr.Content,
			ToolCallID: tr.ToolCallID,
		})
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	oaiReq := openaiRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	// 添加 tools 定义
	if len(req.Tools) > 0 {
		oaiReq.Tools = make([]openaiTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			oaiReq.Tools = append(oaiReq.Tools, openaiTool{
				Type: "function",
				Function: openaiToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if oaiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", oaiResp.Error.Message)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := oaiResp.Choices[0]
	result := &CompletionResponse{
		Model:        oaiResp.Model,
		InputTokens:  oaiResp.Usage.PromptTokens,
		OutputTokens: oaiResp.Usage.CompletionTokens,
		TotalTokens:  oaiResp.Usage.TotalTokens,
		StopReason:   choice.FinishReason,
	}

	if choice.Message.Content != nil {
		result.Content = *choice.Message.Content
	}

	// 解析 tool calls
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}
