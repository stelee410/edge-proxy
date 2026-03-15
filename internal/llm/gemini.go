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
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"
)

// GeminiProvider Google Gemini 原生 API 适配器
type GeminiProvider struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGeminiProvider 创建 Gemini Provider
func NewGeminiProvider(name, baseURL, apiKey, model string) *GeminiProvider {
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	return &GeminiProvider{
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
func (p *GeminiProvider) Name() string {
	return p.name
}

// buildGeminiContents 将 LLM 消息列表转换为 Gemini contents 格式
// 支持多模态、tool calls 历史、tool results
func buildGeminiContents(req *CompletionRequest) []geminiContent {
	contents := make([]geminiContent, 0, len(req.Messages)+1)
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		// assistant 消息含有 tool calls → functionCall parts
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			parts := make([]interface{}, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": tc.Name,
						"args": tc.Arguments,
					},
				})
			}
			contents = append(contents, geminiContent{Role: "model", Parts: parts})
			continue
		}

		// 多模态内容
		if len(m.ContentParts) > 0 {
			rawParts := make([]interface{}, 0, len(m.ContentParts))
			for _, cp := range m.ContentParts {
				if cp.Type == "text" {
					rawParts = append(rawParts, geminiPart{Text: cp.Text})
				} else if cp.Type == "image" && cp.ImageBase64 != "" {
					mime := cp.MimeType
					if mime == "" {
						mime = "image/jpeg"
					}
					rawParts = append(rawParts, map[string]interface{}{
						"inline_data": map[string]string{"mime_type": mime, "data": cp.ImageBase64},
					})
				}
			}
			if len(rawParts) > 0 {
				contents = append(contents, geminiContent{Role: role, Parts: rawParts})
			} else {
				contents = append(contents, geminiContent{Role: role, Parts: []interface{}{geminiPart{Text: m.Content}}})
			}
			continue
		}

		contents = append(contents, geminiContent{Role: role, Parts: []interface{}{geminiPart{Text: m.Content}}})
	}

	// tool results → user message with functionResponse parts
	if len(req.ToolResults) > 0 {
		parts := make([]interface{}, 0, len(req.ToolResults))
		for _, tr := range req.ToolResults {
			parts = append(parts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     tr.ToolCallID, // Gemini 用 name（函数名）而非 call ID，此处复用 ToolCallID 作为 name
					"response": map[string]interface{}{"output": tr.Content},
				},
			})
		}
		contents = append(contents, geminiContent{Role: "user", Parts: parts})
	}

	return contents
}

// buildGeminiTools 将 ToolDefinition 列表转换为 Gemini tools 格式
func buildGeminiTools(tools []ToolDefinition) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	decls := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		decl := map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
		}
		if t.InputSchema != nil {
			decl["parameters"] = t.InputSchema
		}
		decls = append(decls, decl)
	}
	return []map[string]interface{}{{"functionDeclarations": decls}}
}

// parseGeminiResponseParts 解析 Gemini 响应 parts，提取文本和 function calls
func parseGeminiResponseParts(rawParts []json.RawMessage) (text string, toolCalls []ToolCall) {
	for _, raw := range rawParts {
		var part struct {
			Text         string `json:"text"`
			FunctionCall *struct {
				Name string                 `json:"name"`
				Args map[string]interface{} `json:"args"`
			} `json:"functionCall"`
		}
		if err := json.Unmarshal(raw, &part); err != nil {
			continue
		}
		if part.Text != "" {
			text += part.Text
		}
		if part.FunctionCall != nil {
			toolCalls = append(toolCalls, ToolCall{
				ID:        part.FunctionCall.Name, // Gemini 没有 call ID，用 name 代替
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			})
		}
	}
	return
}

// StreamComplete 流式补全 - 调用 Gemini streamGenerateContent API（支持 function calling）
func (p *GeminiProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	contents := buildGeminiContents(req)

	model := req.Model
	if model == "" {
		model = p.model
	}

	geminiReq := map[string]interface{}{
		"contents": contents,
	}
	if req.SystemPrompt != "" {
		geminiReq["systemInstruction"] = map[string]interface{}{
			"parts": []geminiPart{{Text: req.SystemPrompt}},
		}
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		cfg := map[string]interface{}{}
		if req.Temperature > 0 {
			cfg["temperature"] = req.Temperature
		}
		if req.MaxTokens > 0 {
			cfg["maxOutputTokens"] = req.MaxTokens
		}
		geminiReq["generationConfig"] = cfg
	}
	if gTools := buildGeminiTools(req.Tools); gTools != nil {
		geminiReq["tools"] = gTools
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", p.baseURL, model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamEvent, 16)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		var inputTokens, outputTokens int
		var allToolCalls []ToolCall

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamEvent{Error: ctx.Err(), Done: true}
				return
			default:
			}
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			// 用 RawMessage 解析 parts，以便区分 text 和 functionCall
			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []json.RawMessage `json:"parts"`
						Role  string            `json:"role"`
					} `json:"content"`
					FinishReason string `json:"finishReason"`
				} `json:"candidates"`
				UsageMetadata *struct {
					PromptTokenCount     int `json:"promptTokenCount"`
					CandidatesTokenCount int `json:"candidatesTokenCount"`
				} `json:"usageMetadata"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.Error != nil {
				ch <- StreamEvent{Error: fmt.Errorf("Gemini error: %s", chunk.Error.Message), Done: true}
				return
			}
			if chunk.UsageMetadata != nil {
				inputTokens = chunk.UsageMetadata.PromptTokenCount
				outputTokens = chunk.UsageMetadata.CandidatesTokenCount
			}
			if len(chunk.Candidates) > 0 {
				text, tcs := parseGeminiResponseParts(chunk.Candidates[0].Content.Parts)
				if text != "" {
					ch <- StreamEvent{Content: text}
				}
				allToolCalls = append(allToolCalls, tcs...)
			}
		}
		ch <- StreamEvent{
			Done:         true,
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			ToolCalls:    allToolCalls,
		}
	}()

	return ch, nil
}

// Gemini API 请求/响应结构

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string        `json:"role"`
	Parts []interface{} `json:"parts"` // geminiPart、inline_data map、functionCall map、functionResponse map
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

// Complete 实现 Provider 接口 - 调用 Gemini generateContent API（支持 function calling）
func (p *GeminiProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	contents := buildGeminiContents(req)

	model := req.Model
	if model == "" {
		model = p.model
	}

	geminiReq := map[string]interface{}{
		"contents": contents,
	}
	if req.SystemPrompt != "" {
		geminiReq["systemInstruction"] = map[string]interface{}{
			"parts": []geminiPart{{Text: req.SystemPrompt}},
		}
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		cfg := map[string]interface{}{}
		if req.Temperature > 0 {
			cfg["temperature"] = req.Temperature
		}
		if req.MaxTokens > 0 {
			cfg["maxOutputTokens"] = req.MaxTokens
		}
		geminiReq["generationConfig"] = cfg
	}
	if gTools := buildGeminiTools(req.Tools); gTools != nil {
		geminiReq["tools"] = gTools
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.baseURL, model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// 用 RawMessage 解析 parts
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []json.RawMessage `json:"parts"`
				Role  string            `json:"role"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("Gemini API error (%s): %s", geminiResp.Error.Status, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini response")
	}

	content, toolCalls := parseGeminiResponseParts(geminiResp.Candidates[0].Content.Parts)

	result := &CompletionResponse{
		Content:   content,
		Model:     model,
		ToolCalls: toolCalls,
	}

	if geminiResp.UsageMetadata != nil {
		result.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
		result.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		result.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount
	}

	return result, nil
}
