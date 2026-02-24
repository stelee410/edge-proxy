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

// StreamComplete 流式补全（暂未实现）
func (p *GeminiProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("StreamComplete not implemented for Gemini provider %q", p.name)
}

// Gemini API 请求/响应结构

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string        `json:"role"`
	Parts []interface{} `json:"parts"` // geminiPart 或 inline_data map
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

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
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

// Complete 实现 Provider 接口 - 调用 Gemini generateContent API
func (p *GeminiProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构建 contents，将 assistant 角色映射为 model
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if len(m.ContentParts) > 0 {
			rawParts := make([]interface{}, 0, len(m.ContentParts))
			for _, p := range m.ContentParts {
				if p.Type == "text" {
					rawParts = append(rawParts, geminiPart{Text: p.Text})
				} else if p.Type == "image" && p.ImageBase64 != "" {
					mime := p.MimeType
					if mime == "" {
						mime = "image/jpeg"
					}
					rawParts = append(rawParts, map[string]interface{}{
						"inline_data": map[string]string{
							"mime_type": mime,
							"data":      p.ImageBase64,
						},
					})
				}
			}
			if len(rawParts) > 0 {
				contents = append(contents, geminiContent{Role: role, Parts: rawParts})
			} else {
				contents = append(contents, geminiContent{Role: role, Parts: []interface{}{geminiPart{Text: m.Content}}})
			}
		} else {
			contents = append(contents, geminiContent{Role: role, Parts: []interface{}{geminiPart{Text: m.Content}}})
		}
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	geminiReq := geminiRequest{
		Contents: contents,
	}

	// System prompt 作为 systemInstruction
	if req.SystemPrompt != "" {
		geminiReq.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	// Generation config
	if req.Temperature > 0 || req.MaxTokens > 0 {
		geminiReq.GenerationConfig = &geminiGenerationConfig{}
		if req.Temperature > 0 {
			t := req.Temperature
			geminiReq.GenerationConfig.Temperature = &t
		}
		if req.MaxTokens > 0 {
			m := req.MaxTokens
			geminiReq.GenerationConfig.MaxOutputTokens = &m
		}
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

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("Gemini API error (%s): %s", geminiResp.Error.Status, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini response")
	}

	// 拼接所有 parts 的文本
	var content string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		content += part.Text
	}

	result := &CompletionResponse{
		Content: content,
		Model:   model,
	}

	if geminiResp.UsageMetadata != nil {
		result.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
		result.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		result.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount
	}

	return result, nil
}
