package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultClaudeBaseURL = "https://api.anthropic.com"
	claudeAPIVersion     = "2023-06-01"
)

// ClaudeProvider Anthropic Claude 原生 API 适配器
type ClaudeProvider struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClaudeProvider 创建 Claude Provider
func NewClaudeProvider(name, baseURL, apiKey, model string) *ClaudeProvider {
	if baseURL == "" {
		baseURL = defaultClaudeBaseURL
	}
	return &ClaudeProvider{
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
func (p *ClaudeProvider) Name() string {
	return p.name
}

// StreamComplete 流式补全 - 调用 Anthropic Messages API（stream=true）
func (p *ClaudeProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	messages := make([]claudeMessage, 0, len(req.Messages)+len(req.ToolResults))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			blocks := make([]claudeContentBlock, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				blocks = append(blocks, claudeContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, claudeContentBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: tc.Arguments})
			}
			messages = append(messages, claudeMessage{Role: "assistant", Content: blocks})
		} else if len(m.ContentParts) > 0 {
			contentBlocks := make([]interface{}, 0, len(m.ContentParts))
			var lastText string
			for _, cp := range m.ContentParts {
				if cp.Type == "text" {
					lastText = cp.Text
				} else if cp.Type == "image" && cp.ImageBase64 != "" {
					mime := cp.MimeType
					if mime == "" {
						mime = "image/jpeg"
					}
					contentBlocks = append(contentBlocks, map[string]interface{}{
						"type": "image",
						"source": map[string]string{"type": "base64", "media_type": mime, "data": cp.ImageBase64},
					})
				}
			}
			if lastText != "" {
				contentBlocks = append(contentBlocks, map[string]interface{}{"type": "text", "text": lastText})
			}
			if len(contentBlocks) > 0 {
				messages = append(messages, claudeMessage{Role: m.Role, Content: contentBlocks})
			} else {
				messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
			}
		} else {
			messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
		}
	}
	if len(req.ToolResults) > 0 {
		blocks := make([]claudeContentBlock, 0, len(req.ToolResults))
		for _, tr := range req.ToolResults {
			blocks = append(blocks, claudeContentBlock{Type: "tool_result", ToolUseID: tr.ToolCallID, Content: tr.Content, IsError: tr.IsError})
		}
		messages = append(messages, claudeMessage{Role: "user", Content: blocks})
	}

	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	claudeReq := claudeRequest{Model: model, MaxTokens: maxTokens, System: req.SystemPrompt, Messages: messages}
	if req.Temperature > 0 {
		claudeReq.Temperature = &req.Temperature
	}
	if len(req.Tools) > 0 {
		claudeReq.Tools = make([]claudeTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			claudeReq.Tools = append(claudeReq.Tools, claudeTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
		}
	}

	body, err := json.Marshal(struct {
		claudeRequest
		Stream bool `json:"stream"`
	}{claudeReq, true})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamEvent, 16)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		var inputTokens, outputTokens int
		var respModel string

		type partialToolUse struct {
			id    string
			name  string
			input strings.Builder
		}
		toolUseMap := map[int]*partialToolUse{}

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamEvent{Error: ctx.Err(), Done: true}
				return
			default:
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var evt struct {
				Type         string `json:"type"`
				Index        int    `json:"index"`
				ContentBlock *struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block,omitempty"`
				Delta *struct {
					Type        string `json:"type"`
					Text        string `json:"text,omitempty"`
					PartialJSON string `json:"partial_json,omitempty"`
				} `json:"delta,omitempty"`
				Usage *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage,omitempty"`
				Message *struct {
					Model string `json:"model,omitempty"`
					Usage *struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage,omitempty"`
				} `json:"message,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}

			if evt.Message != nil {
				if evt.Message.Model != "" {
					respModel = evt.Message.Model
				}
				if evt.Message.Usage != nil {
					inputTokens = evt.Message.Usage.InputTokens
					outputTokens = evt.Message.Usage.OutputTokens
				}
			}
			if evt.Usage != nil {
				if evt.Usage.InputTokens > 0 {
					inputTokens = evt.Usage.InputTokens
				}
				if evt.Usage.OutputTokens > 0 {
					outputTokens = evt.Usage.OutputTokens
				}
			}

			switch evt.Type {
			case "content_block_start":
				if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
					toolUseMap[evt.Index] = &partialToolUse{id: evt.ContentBlock.ID, name: evt.ContentBlock.Name}
				}
			case "content_block_delta":
				if evt.Delta != nil {
					if evt.Delta.Type == "text_delta" && evt.Delta.Text != "" {
						ch <- StreamEvent{Content: evt.Delta.Text}
					}
					if evt.Delta.Type == "input_json_delta" && evt.Delta.PartialJSON != "" {
						if tu, ok := toolUseMap[evt.Index]; ok {
							tu.input.WriteString(evt.Delta.PartialJSON)
						}
					}
				}
			case "message_delta", "message_stop":
				var toolCalls []ToolCall
				for i := 0; ; i++ {
					tu, ok := toolUseMap[i]
					if !ok {
						break
					}
					inputStr := tu.input.String()
					if inputStr == "" {
						inputStr = "{}"
					}
					var args map[string]interface{}
					json.Unmarshal([]byte(inputStr), &args)
					toolCalls = append(toolCalls, ToolCall{ID: tu.id, Name: tu.name, Arguments: args})
				}
				ch <- StreamEvent{
					Done:         true,
					Model:        respModel,
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
					ToolCalls:    toolCalls,
				}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: fmt.Errorf("stream read error: %w", err), Done: true}
		}
	}()

	return ch, nil
}

// Anthropic Messages API 请求/响应结构

type claudeRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens"`
	System      string              `json:"system,omitempty"`
	Messages    []claudeMessage     `json:"messages"`
	Temperature *float64            `json:"temperature,omitempty"`
	Tools       []claudeTool        `json:"tools,omitempty"`
}

type claudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string 或 []claudeContentBlock
}

type claudeContentBlock struct {
	Type      string                 `json:"type"`                  // "text", "tool_use", "tool_result"
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`          // tool_use ID
	Name      string                 `json:"name,omitempty"`        // tool name
	Input     map[string]interface{} `json:"input,omitempty"`       // tool input
	ToolUseID string                 `json:"tool_use_id,omitempty"` // for tool_result
	Content   string                 `json:"content,omitempty"`     // for tool_result
	IsError   bool                   `json:"is_error,omitempty"`    // for tool_result
}

type claudeTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type  string                 `json:"type"` // "text" or "tool_use"
		Text  string                 `json:"text,omitempty"`
		ID    string                 `json:"id,omitempty"`
		Name  string                 `json:"name,omitempty"`
		Input map[string]interface{} `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"` // "end_turn", "tool_use", etc.
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete 实现 Provider 接口 - 调用 Anthropic Messages API
func (p *ClaudeProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	messages := make([]claudeMessage, 0, len(req.Messages)+len(req.ToolResults))

	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}
		// assistant 消息可能带有 tool_calls，需要转换为 Claude 的 tool_use content blocks
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			blocks := make([]claudeContentBlock, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				blocks = append(blocks, claudeContentBlock{
					Type: "text",
					Text: m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, claudeContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Arguments,
				})
			}
			messages = append(messages, claudeMessage{
				Role:    "assistant",
				Content: blocks,
			})
		} else if len(m.ContentParts) > 0 {
			contentBlocks := make([]interface{}, 0, len(m.ContentParts))
			var lastText string
			for _, p := range m.ContentParts {
				if p.Type == "text" {
					lastText = p.Text
				} else if p.Type == "image" && p.ImageBase64 != "" {
					mime := p.MimeType
					if mime == "" {
						mime = "image/jpeg"
					}
					contentBlocks = append(contentBlocks, map[string]interface{}{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": mime,
							"data":       p.ImageBase64,
						},
					})
				}
			}
			if lastText != "" {
				contentBlocks = append(contentBlocks, map[string]interface{}{"type": "text", "text": lastText})
			}
			if len(contentBlocks) > 0 {
				messages = append(messages, claudeMessage{Role: m.Role, Content: contentBlocks})
			} else {
				messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
			}
		} else {
			messages = append(messages, claudeMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	// 添加 tool results（作为 user 消息中的 tool_result 内容块）
	if len(req.ToolResults) > 0 {
		blocks := make([]claudeContentBlock, 0, len(req.ToolResults))
		for _, tr := range req.ToolResults {
			blocks = append(blocks, claudeContentBlock{
				Type:      "tool_result",
				ToolUseID: tr.ToolCallID,
				Content:   tr.Content,
				IsError:   tr.IsError,
			})
		}
		messages = append(messages, claudeMessage{
			Role:    "user",
			Content: blocks,
		})
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	claudeReq := claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
	}
	if req.Temperature > 0 {
		claudeReq.Temperature = &req.Temperature
	}

	// 添加 tools 定义
	if len(req.Tools) > 0 {
		claudeReq.Tools = make([]claudeTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			claudeReq.Tools = append(claudeReq.Tools, claudeTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

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
		return nil, fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if claudeResp.Error != nil {
		return nil, fmt.Errorf("Anthropic API error (%s): %s", claudeResp.Error.Type, claudeResp.Error.Message)
	}

	result := &CompletionResponse{
		Model:        claudeResp.Model,
		InputTokens:  claudeResp.Usage.InputTokens,
		OutputTokens: claudeResp.Usage.OutputTokens,
		TotalTokens:  claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		StopReason:   claudeResp.StopReason,
	}

	// 解析内容块
	var textContent string
	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}
	result.Content = textContent

	return result, nil
}
