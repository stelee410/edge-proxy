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

// OllamaProvider Ollama 原生 API 适配器
type OllamaProvider struct {
	name       string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider 创建 Ollama Provider
func NewOllamaProvider(name, baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		name:    name,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name 返回 Provider 名称
func (p *OllamaProvider) Name() string {
	return p.name
}

// StreamComplete 流式补全（暂未实现）
func (p *OllamaProvider) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("StreamComplete not implemented for Ollama provider %q", p.name)
}

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  *ollamaOptions   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // 多模态：base64 编码的图片
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done               bool `json:"done"`
	TotalDuration      int  `json:"total_duration"`
	PromptEvalCount    int  `json:"prompt_eval_count"`
	EvalCount          int  `json:"eval_count"`
	Error              string `json:"error,omitempty"`
}

// Complete 实现 Provider 接口
func (p *OllamaProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		om := ollamaMessage{Role: m.Role, Content: m.Content}
		if len(m.ContentParts) > 0 {
			var textParts []string
			for _, p := range m.ContentParts {
				if p.Type == "text" {
					textParts = append(textParts, p.Text)
				} else if p.Type == "image" && p.ImageBase64 != "" {
					om.Images = append(om.Images, p.ImageBase64)
				}
			}
			if len(textParts) > 0 {
				om.Content = textParts[0]
				for _, t := range textParts[1:] {
					om.Content += "\n" + t
				}
			}
			if om.Content == "" && len(om.Images) > 0 {
				om.Content = "请分析以上图片"
			}
		}
		messages = append(messages, om)
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	ollamaReq := ollamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/api/chat"
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
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", ollamaResp.Error)
	}

	return &CompletionResponse{
		Content:      ollamaResp.Message.Content,
		Model:        ollamaResp.Model,
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
		TotalTokens:  ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}, nil
}
