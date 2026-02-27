package skills

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func init() {
	RegisterCodeHandler("web_search", NewWebSearchHandler)
}

// WebSearchHandler 根据用户需求进行网络搜索，抓取页面内容并整理成文字报告
type WebSearchHandler struct {
	client *http.Client
}

// NewWebSearchHandler 创建网络搜索 Handler
func NewWebSearchHandler(config map[string]interface{}, globalCfg map[string]interface{}) (CodeHandler, error) {
	timeout := 25 * time.Second
	if t, ok := config["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}
	return &WebSearchHandler{
		client: &http.Client{Timeout: timeout},
	}, nil
}

// Execute 执行网络搜索：DuckDuckGo HTML 获取结果链接 → 抓取各页面 → 整理成报告
func (h *WebSearchHandler) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	query := ""
	maxResults := 5
	maxCharsPerPage := 6000

	if input.Arguments != nil {
		if q, ok := input.Arguments["query"].(string); ok {
			query = strings.TrimSpace(q)
		}
		if n, ok := input.Arguments["max_results"].(float64); ok && n > 0 && n <= 10 {
			maxResults = int(n)
		}
		if n, ok := input.Arguments["max_results"].(int); ok && n > 0 && n <= 10 {
			maxResults = n
		}
		if c, ok := input.Arguments["max_chars_per_page"].(float64); ok && c > 0 {
			maxCharsPerPage = int(c)
		}
		if c, ok := input.Arguments["max_chars_per_page"].(int); ok && c > 0 {
			maxCharsPerPage = c
		}
	}

	if query == "" {
		return &SkillOutput{
			Success: false,
			Error:   "请提供搜索关键词（query）",
		}, nil
	}

	// 1. DuckDuckGo HTML 搜索获取结果链接
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return &SkillOutput{Success: false, Error: fmt.Sprintf("创建请求失败: %v", err)}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := h.client.Do(req)
	if err != nil {
		return &SkillOutput{Success: false, Error: fmt.Sprintf("搜索请求失败: %v", err)}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("搜索返回状态码: %d", resp.StatusCode),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &SkillOutput{Success: false, Error: fmt.Sprintf("读取搜索响应失败: %v", err)}, err
	}

	// 解析 DDG 结果中的真实 URL（uddg= 后的编码链接）
	urls := parseDuckDuckGoResultURLs(string(body), maxResults)
	if len(urls) == 0 {
		return &SkillOutput{
			Success: true,
			Content: "未找到与「" + query + "」相关的网页结果，请尝试其他关键词。",
		}, nil
	}

	// 2. 逐个抓取页面并提取正文
	var sections []string
	sections = append(sections, fmt.Sprintf("# 网络搜索报告：%s\n", query))
	sections = append(sections, fmt.Sprintf("共检索 %d 条结果，以下为抓取并整理后的内容。\n", len(urls)))

	for i, u := range urls {
		title, content, err := h.fetchPageContent(ctx, u, maxCharsPerPage)
		if err != nil {
			sections = append(sections, fmt.Sprintf("## %d. %s\n链接: %s\n\n（抓取失败: %v）\n", i+1, u, u, err))
			continue
		}
		if title == "" {
			title = u
		}
		content = strings.TrimSpace(content)
		if len(content) > maxCharsPerPage {
			content = content[:maxCharsPerPage] + "..."
		}
		sections = append(sections, fmt.Sprintf("## %d. %s\n链接: %s\n\n%s\n\n---\n", i+1, title, u, content))
	}

	return &SkillOutput{
		Success: true,
		Content: strings.Join(sections, "\n"),
	}, nil
}

// parseDuckDuckGoResultURLs 从 DDG HTML 结果页解析出真实 URL（解码 uddg 参数），去重并限制数量
func parseDuckDuckGoResultURLs(html string, max int) []string {
	re := regexp.MustCompile(`uddg=([^&"'\s]+)`)
	matches := re.FindAllStringSubmatch(html, -1)
	seen := make(map[string]bool)
	var urls []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		decoded, err := url.QueryUnescape(m[1])
		if err != nil || decoded == "" {
			continue
		}
		if !strings.HasPrefix(decoded, "http://") && !strings.HasPrefix(decoded, "https://") {
			continue
		}
		if strings.Contains(decoded, "duckduckgo.com") {
			continue
		}
		if seen[decoded] {
			continue
		}
		seen[decoded] = true
		urls = append(urls, decoded)
		if len(urls) >= max {
			break
		}
	}
	return urls
}

// fetchPageContent 抓取单页，返回页面标题和正文摘要（已转纯文本/简单 Markdown）
func (h *WebSearchHandler) fetchPageContent(ctx context.Context, pageURL string, maxChars int) (title, content string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	html := string(body)
	title = extractTitle(html)
	content = htmlToMarkdown(html)
	return title, content, nil
}

// extractTitle 从 HTML 中提取 <title>
func extractTitle(html string) string {
	re := regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
