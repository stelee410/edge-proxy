package tui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/quick"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"
)

// Highlighter 语法高亮器
type Highlighter struct {
	style     *chroma.Style
	formatter *chroma.Formatter
	enabled   bool
	theme     string
}

// NewHighlighter 创建语法高亮器
func NewHighlighter(theme string) *Highlighter {
	h := &Highlighter{
		enabled: true,
		theme:   theme,
	}

	// 加载主题
	if theme == "light" {
		h.style = styles.GitHub
	} else {
		h.style = styles.Monokai
	}

	// 使用 quick.Highlight，不需要手动创建 formatter
	h.formatter = nil

	return h
}

// Highlight 高亮代码
func (h *Highlighter) Highlight(code string, language string) (string, error) {
	if !h.enabled || language == "" || language == "text" {
		return code, nil
	}

	// 规范化语言名称
	lang := normalizeLanguage(language)

	var sb strings.Builder
	err := quick.Highlight(&sb, code, lang, "terminal256", h.theme)
	if err != nil {
		// 如果高亮失败，返回原始代码
		return code, nil
	}

	return sb.String(), nil
}

// HighlightLines 高亮多行代码
func (h *Highlighter) HighlightLines(code string, language string, lineNum bool) (string, error) {
	highlighted, err := h.Highlight(code, language)
	if err != nil {
		return "", err
	}

	if !lineNum {
		return highlighted, nil
	}

	// 添加行号
	lines := strings.Split(highlighted, "\n")
	var sb strings.Builder
	maxLineNum := len(lines)
	width := len(fmt.Sprintf("%d", maxLineNum))

	for i, line := range lines {
		lineNumStr := fmt.Sprintf("%*d", width, i+1)
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(lineNumStr))
		sb.WriteString(" ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// SetEnabled 启用/禁用高亮
func (h *Highlighter) SetEnabled(enabled bool) {
	h.enabled = enabled
}

// SetTheme 设置主题
func (h *Highlighter) SetTheme(theme string) {
	h.theme = theme
	if theme == "light" {
		h.style = styles.GitHub
	} else {
		h.style = styles.Monokai
	}
}

// normalizeLanguage 规范化语言名称
func normalizeLanguage(lang string) string {
	lang = strings.ToLower(lang)

	// 语言别名映射
	aliases := map[string]string{
		"js":         "javascript",
		"ts":         "typescript",
		"py":         "python",
		"rb":         "ruby",
		"go":         "go",
		"java":       "java",
		"c":          "c",
		"cpp":        "cpp",
		"c++":        "cpp",
		"h":          "c",
		"hpp":        "cpp",
		"rs":         "rust",
		"sh":         "bash",
		"bash":       "bash",
		"zsh":        "bash",
		"yaml":       "yaml",
		"yml":        "yaml",
		"json":       "json",
		"xml":        "xml",
		"html":       "html",
		"css":        "css",
		"scss":       "scss",
		"sql":        "sql",
		"md":         "markdown",
		"markdown":   "markdown",
		"txt":        "text",
		"dockerfile": "docker",
	}

	if alias, ok := aliases[lang]; ok {
		return alias
	}

	return lang
}

// DetectLanguage 检测代码语言
func DetectLanguage(filename string) string {
	ext := strings.ToLower(filename)
	dotIdx := strings.LastIndex(ext, ".")
	if dotIdx >= 0 {
		ext = ext[dotIdx:]
	}

	// 扩展名映射
	extensions := map[string]string{
		".go":    "go",
		".js":    "javascript",
		".ts":    "typescript",
		".tsx":   "tsx",
		".jsx":   "jsx",
		".py":    "python",
		".rb":    "ruby",
		".java":  "java",
		".c":     "c",
		".h":     "c",
		".cpp":   "cpp",
		".hpp":   "cpp",
		".cc":    "cpp",
		".cxx":   "cpp",
		".rs":    "rust",
		".sh":    "bash",
		".bash":  "bash",
		".zsh":   "bash",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".xml":   "xml",
		".html":  "html",
		".htm":   "html",
		".css":   "css",
		".scss":  "scss",
		".sass":  "scss",
		".sql":   "sql",
		".md":    "markdown",
		".txt":   "text",
		".toml":  "toml",
		".ini":   "ini",
		".conf":  "ini",
	}

	if lang, ok := extensions[ext]; ok {
		return lang
	}

	return "text"
}
