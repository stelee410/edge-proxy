package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// APIConfig Prompt-API Skill 的 API 配置
type APIConfig struct {
	URL              string            `yaml:"url"`
	Method           string            `yaml:"method"`
	Headers          map[string]string `yaml:"headers"`
	BodyTemplate     string            `yaml:"body_template"`
	ResponseTemplate string            `yaml:"response_template"`
}

// PromptAPISkill 基于 API 调用的 Skill 实现
// 根据配置发起 HTTP 请求，解析响应并返回结果
type PromptAPISkill struct {
	config     SkillConfig
	httpClient *HTTPClient
}

// NewPromptAPISkill 从配置创建 Prompt-API Skill
func NewPromptAPISkill(cfg SkillConfig) *PromptAPISkill {
	return &PromptAPISkill{
		config:     cfg,
		httpClient: NewHTTPClient(),
	}
}

// Name 返回 Skill 名称
func (s *PromptAPISkill) Name() string { return s.config.Name }

// Stage 返回执行阶段
func (s *PromptAPISkill) Stage() string { return s.config.Stage }

// Type 返回实现类型
func (s *PromptAPISkill) Type() string { return TypePromptAPI }

// Definition 返回 Skill 定义
func (s *PromptAPISkill) Definition() SkillDefinition {
	return s.config.ToDefinition()
}

// Execute 执行 Prompt-API Skill：调用外部 API 并返回结果
func (s *PromptAPISkill) Execute(_ context.Context, input *SkillInput) (*SkillOutput, error) {
	if s.config.APIURL == "" {
		return &SkillOutput{
			Success: false,
			Error:   "api_url is not configured",
		}, fmt.Errorf("api_url is not configured for skill %q", s.config.Name)
	}

	// 准备模板数据
	data := make(map[string]interface{})
	if input != nil && input.Arguments != nil {
		for k, v := range input.Arguments {
			data[k] = v
		}
	}
	if input != nil && input.Context != nil {
		for k, v := range input.Context {
			data[k] = v
		}
	}

	// 渲染 URL
	url, err := RenderTemplate(s.config.APIURL, data)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to render URL template: %v", err),
		}, err
	}

	// 方法
	method := strings.ToUpper(s.config.APIMethod)
	if method == "" {
		method = "GET"
	}

	// 渲染请求头
	headers := make(map[string]string)
	for k, v := range s.config.APIHeaders {
		rendered, err := RenderTemplate(v, data)
		if err != nil {
			headers[k] = v
		} else {
			headers[k] = rendered
		}
	}

	// 渲染请求体（如有）
	var body string
	if s.config.APIHeaders != nil {
		// 检查是否有 body_template（通过 SkillConfig 扩展）
		if bodyTmpl, ok := data["_body_template"].(string); ok && bodyTmpl != "" {
			body, _ = RenderTemplate(bodyTmpl, data)
		}
	}

	// 如果是 POST/PUT 且没有 body，把 arguments 作为 JSON body
	if (method == "POST" || method == "PUT") && body == "" && len(data) > 0 {
		bodyBytes, _ := json.Marshal(input.Arguments)
		body = string(bodyBytes)
	}

	// 发起请求
	respBody, statusCode, err := s.httpClient.Do(method, url, body, headers)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("API request failed: %v", err),
		}, err
	}

	if statusCode >= 400 {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("API returned status %d: %s", statusCode, truncate(respBody, 500)),
		}, nil
	}

	// 如果有 response_template，渲染响应
	content := respBody
	if s.config.APIHeaders != nil {
		// 尝试将 JSON 响应解析为 map，用于模板渲染
		var respData map[string]interface{}
		if json.Unmarshal([]byte(respBody), &respData) == nil {
			// 合并响应数据到模板数据
			for k, v := range respData {
				data[k] = v
			}
		}
	}

	return &SkillOutput{
		Content: content,
		Success: true,
		Metadata: map[string]interface{}{
			"skill_name":  s.config.Name,
			"skill_type":  TypePromptAPI,
			"status_code": statusCode,
		},
	}, nil
}

// truncate 截断字符串到指定长度
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
