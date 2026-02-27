package skills

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// init 注册 handler
func init() {
	RegisterCodeHandler("trending_github", NewTrendingGitHubHandler)
	RegisterCodeHandler("trending_hackernews", NewTrendingHackerNewsHandler)
}

// TrendingGitHubHandler GitHub 趋势仓库 Handler
type TrendingGitHubHandler struct {
	client *http.Client
}

// NewTrendingGitHubHandler 创建 GitHub 趋势 Handler
func NewTrendingGitHubHandler(config map[string]interface{}, globalCfg map[string]interface{}) (CodeHandler, error) {
	timeout := 30 * time.Second
	if t, ok := config["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Second
	}
	return &TrendingGitHubHandler{
		client: &http.Client{Timeout: timeout},
	}, nil
}

// Execute 执行抓取
func (h *TrendingGitHubHandler) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	// 获取参数
	language := ""
	if input.Arguments != nil {
		if lang, ok := input.Arguments["language"].(string); ok {
			language = strings.TrimSpace(lang)
		}
		if lang, ok := input.Arguments["lang"].(string); ok {
			language = strings.TrimSpace(lang)
		}
	}

	// 构建 URL
	url := "https://github.com/trending"
	if language != "" {
		url += "/" + strings.ToLower(language)
	}

	// 发起请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("创建请求失败: %v", err),
		}, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := h.client.Do(req)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("请求失败: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("返回状态码: %d", resp.StatusCode),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("读取响应失败: %v", err),
		}, err
	}

	// 直接返回原始 HTML 转换为 Markdown
	// 简单的 HTML 转 Markdown
	markdown := htmlToMarkdown(string(body))

	return &SkillOutput{
		Success: true,
		Content: markdown,
	}, nil
}

// TrendingHackerNewsHandler Hacker News 趋势 Handler
type TrendingHackerNewsHandler struct {
	client *http.Client
}

// NewTrendingHackerNewsHandler 创建 Hacker News 趋势 Handler
func NewTrendingHackerNewsHandler(config map[string]interface{}, globalCfg map[string]interface{}) (CodeHandler, error) {
	timeout := 30 * time.Second
	if t, ok := config["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Second
	}
	return &TrendingHackerNewsHandler{
		client: &http.Client{Timeout: timeout},
	}, nil
}

// Execute 执行抓取
func (h *TrendingHackerNewsHandler) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	// 直接从 hackernews.com 抓取数据
	url := "https://hackernews.com"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("创建请求失败: %v", err),
		}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := h.client.Do(req)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("请求失败: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("返回状态码: %d", resp.StatusCode),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   fmt.Sprintf("读取响应失败: %v", err),
		}, err
	}

	// 直接返回原始 HTML 转换为 Markdown
	markdown := htmlToMarkdown(string(body))

	return &SkillOutput{
		Success: true,
		Content: markdown,
	}, nil
}

// htmlToMarkdown 简单的 HTML 转 Markdown
func htmlToMarkdown(html string) string {
	// 这是一个非常简单的转换，保留主要内容结构
	// 移除 script 和 style 标签
	html = removeTags(html, "script")
	html = removeTags(html, "style")
	html = removeTags(html, "noscript")

	// 简单的标题转换
	html = strings.ReplaceAll(html, "<h1>", "# ")
	html = strings.ReplaceAll(html, "</h1>", "\n\n")
	html = strings.ReplaceAll(html, "<h2>", "## ")
	html = strings.ReplaceAll(html, "</h2>", "\n\n")
	html = strings.ReplaceAll(html, "<h3>", "### ")
	html = strings.ReplaceAll(html, "</h3>", "\n\n")

	// 粗体和斜体
	html = strings.ReplaceAll(html, "<strong>", "**")
	html = strings.ReplaceAll(html, "</strong>", "**")
	html = strings.ReplaceAll(html, "<b>", "**")
	html = strings.ReplaceAll(html, "</b>", "**")
	html = strings.ReplaceAll(html, "<em>", "*")
	html = strings.ReplaceAll(html, "</em>", "*")
	html = strings.ReplaceAll(html, "<i>", "*")
	html = strings.ReplaceAll(html, "</i>", "*")

	// 链接
	// 简单处理：保留 href 和文本内容
	// 对于复杂的 a 标签，暂时保留原样

	// 段落和换行
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<p>", "\n\n")
	html = strings.ReplaceAll(html, "</p>", "\n")
	html = strings.ReplaceAll(html, "<div>", "\n")
	html = strings.ReplaceAll(html, "</div>", "\n")

	// 列表
	html = strings.ReplaceAll(html, "<ul>", "\n\n")
	html = strings.ReplaceAll(html, "</ul>", "\n\n")
	html = strings.ReplaceAll(html, "<li>", "- ")
	html = strings.ReplaceAll(html, "</li>", "\n")

	// 移除所有剩余的 HTML 标签
	re := getHTMLEntityRemover()
	html = re.ReplaceAllString(html, "")

	// 清理多余的空行
	lines := strings.Split(html, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}

// removeTags 移除指定标签
func removeTags(html, tag string) string {
	// 移除 <tag...> 和 </tag>
	re := getTagRemover(tag)
	html = re.ReplaceAllString(html, "")

	// 移除 <tag/> 形式的自闭合标签
	selfClosingTag := "<" + tag + "[^>]*/>"
	selfCloseRe := getPatternRemover(selfClosingTag)
	html = selfCloseRe.ReplaceAllString(html, "")

	return html
}

var (
	tagRemoverCache = make(map[string]*regexp.Regexp)
	entityRemover *regexp.Regexp
)

// getTagRemover 获取标签移除正则表达式（带缓存）
func getTagRemover(tag string) *regexp.Regexp {
	if re, ok := tagRemoverCache[tag]; ok {
		return re
	}
	pattern := "<" + tag + `[^>]*>|</` + tag + ">"
	re := regexp.MustCompile(pattern)
	tagRemoverCache[tag] = re
	return re
}

// getPatternRemover 获取模式移除正则表达式
func getPatternRemover(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// getHTMLEntityRemover 获取 HTML 实体移除正则表达式
func getHTMLEntityRemover() *regexp.Regexp {
	if entityRemover != nil {
		return entityRemover
	}
	entityRemover = regexp.MustCompile(`<[^>]+>`)
	return entityRemover
}
