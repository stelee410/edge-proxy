package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MarkdownRenderer Markdown 渲染器
type MarkdownRenderer struct {
	styles *ThemeStyles
}

// NewMarkdownRenderer 创建 Markdown 渲染器
func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{
		styles: &GlobalTheme.Styles,
	}
}

// Render 渲染 Markdown
func (r *MarkdownRenderer) Render(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var result strings.Builder

	for _, line := range lines {
		result.WriteString(r.renderLine(line))
		result.WriteString("\n")
	}

	return result.String()
}

// renderLine 渲染单行
func (r *MarkdownRenderer) renderLine(line string) string {
	line = strings.TrimSpace(line)

	// 空行
	if line == "" {
		return ""
	}

	// 标题
	if strings.HasPrefix(line, "#") {
		return r.renderHeading(line)
	}

	// 列表
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return r.renderListItem(line)
	}

	// 数字列表
	if matched, num := r.isNumberedList(line); matched {
		return r.renderNumberedListItem(line, num)
	}

	// 代码块
	if strings.HasPrefix(line, "```") {
		return "" // 代码块由高亮器处理
	}

	// 内联代码
	line = r.renderInlineCode(line)

	// 粗体
	line = r.renderBold(line)

	// 斜体
	line = r.renderItalic(line)

	// 链接
	line = r.renderLink(line)

	return line
}

// renderHeading 渲染标题
func (r *MarkdownRenderer) renderHeading(line string) string {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}

	if level == 0 || level > 6 {
		return line
	}

	text := strings.TrimSpace(line[level:])

	var style lipgloss.Style
	switch level {
	case 1:
		style = lipgloss.NewStyle().
			Foreground(r.styles.Title.GetForeground()).
			Bold(true).
			Underline(true).
			Padding(0, 1)
	case 2:
		style = lipgloss.NewStyle().
			Foreground(r.styles.Command.GetForeground()).
			Bold(true).
			Padding(0, 1)
	default:
		style = lipgloss.NewStyle().
			Foreground(r.styles.Content.GetForeground()).
			Bold(true).
			Padding(0, 1)
	}

	return style.Render(text)
}

// renderListItem 渲染列表项
func (r *MarkdownRenderer) renderListItem(line string) string {
	text := strings.TrimSpace(line[2:])
	style := lipgloss.NewStyle().
		Foreground(r.styles.Content.GetForeground()).
		PaddingLeft(2)

	bullet := lipgloss.NewStyle().
		Foreground(r.styles.Content.GetForeground()).
		Render("•")

	return bullet + " " + style.Render(text)
}

// renderNumberedListItem 渲染数字列表项
func (r *MarkdownRenderer) renderNumberedListItem(line string, num int) string {
	idx := len(r.getNumberString(num))
	text := strings.TrimSpace(line[idx+2:])

	style := lipgloss.NewStyle().
		Foreground(r.styles.Content.GetForeground()).
		PaddingLeft(2)

	number := lipgloss.NewStyle().
		Foreground(r.styles.Content.GetForeground()).
		Render(r.getNumberString(num))

	return number + ". " + style.Render(text)
}

// isNumberedList 检查是否是数字列表
func (r *MarkdownRenderer) isNumberedList(line string) (bool, int) {
	// 匹配 "1. "、"10. " 等
	idx := 0
	for idx < len(line) && line[idx] >= '0' && line[idx] <= '9' {
		idx++
	}

	if idx == 0 || idx > len(line)-2 {
		return false, 0
	}

	if line[idx] == '.' && line[idx+1] == ' ' {
		// 解析数字
		num := 0
		for i := 0; i < idx; i++ {
			num = num*10 + int(line[i]-'0')
		}
		return true, num
	}

	return false, 0
}

// getNumberString 获取数字字符串
func (r *MarkdownRenderer) getNumberString(num int) string {
	result := ""
	for num > 0 {
		result = string(rune('0'+num%10)) + result
		num /= 10
	}
	if result == "" {
		result = "0"
	}
	return result
}

// renderInlineCode 渲染内联代码
func (r *MarkdownRenderer) renderInlineCode(line string) string {
	style := lipgloss.NewStyle().
		Foreground(r.styles.Content.GetForeground()).
		Background(r.styles.CodeBlock.GetBackground()).
		Padding(0, 1)

	// 查找 `code` 模式
	result := line
	start := 0

	for {
		idx := strings.Index(result[start:], "`")
		if idx == -1 {
			break
		}

		pos := start + idx

		// 查找结束的反引号
		endIdx := strings.Index(result[pos+1:], "`")
		if endIdx == -1 {
			break
		}

		end := pos + endIdx + 2

		// 提取代码
		code := result[pos+1 : end-1]

		// 替换为样式化版本
		result = result[:pos] + style.Render(code) + result[end:]

		// 跳过已处理部分
		start = end
	}

	return result
}

// renderBold 渲染粗体
func (r *MarkdownRenderer) renderBold(line string) string {
	// 查找 **bold** 模式
	result := line
	start := 0

	for {
		idx := strings.Index(result[start:], "**")
		if idx == -1 {
			break
		}

		pos := start + idx

		// 查找结束的 **
		endIdx := strings.Index(result[pos+2:], "**")
		if endIdx == -1 {
			break
		}

		end := pos + endIdx + 4

		// 提取文本
		text := result[pos+2 : end-2]

		// 替换为样式化版本
		style := lipgloss.NewStyle().Bold(true)
		result = result[:pos] + style.Render(text) + result[end:]

		// 跳过已处理部分
		start = end
	}

	// 查找 __bold__ 模式
	start = 0
	for {
		idx := strings.Index(result[start:], "__")
		if idx == -1 {
			break
		}

		pos := start + idx

		// 查找结束的 __
		endIdx := strings.Index(result[pos+2:], "__")
		if endIdx == -1 {
			break
		}

		end := pos + endIdx + 4

		// 提取文本
		text := result[pos+2 : end-2]

		// 替换为样式化版本
		style := lipgloss.NewStyle().Bold(true)
		result = result[:pos] + style.Render(text) + result[end:]

		// 跳过已处理部分
		start = end
	}

	return result
}

// renderItalic 渲染斜体
func (r *MarkdownRenderer) renderItalic(line string) string {
	// 查找 *italic* 模式（注意：**bold** 已被处理）
	result := line
	start := 0

	for {
		idx := strings.Index(result[start:], "*")
		if idx == -1 {
			break
		}

		pos := start + idx

		// 检查是否是 **（已经被处理）
		if pos < len(result)-1 && result[pos+1] == '*' {
			start = pos + 2
			continue
		}

		// 查找结束的 *
		endIdx := strings.Index(result[pos+1:], "*")
		if endIdx == -1 {
			break
		}

		// 检查是否是 **（已经被处理）
		endPos := pos + endIdx + 1
		if endPos < len(result)-1 && result[endPos+1] == '*' {
			start = endPos + 2
			continue
		}

		end := endPos + 1

		// 提取文本
		text := result[pos+1 : end]

		// 替换为样式化版本
		style := lipgloss.NewStyle().Italic(true)
		result = result[:pos] + style.Render(text) + result[end:]

		// 跳过已处理部分
		start = end
	}

	return result
}

// renderLink 渲染链接
func (r *MarkdownRenderer) renderLink(line string) string {
	// 查找 [text](url) 模式
	result := line
	start := 0

	for {
		idx := strings.Index(result[start:], "[")
		if idx == -1 {
			break
		}

		pos := start + idx

		// 查找 ]
		endIdx := strings.Index(result[pos+1:], "]")
		if endIdx == -1 {
			break
		}

		textEnd := pos + endIdx + 1

		// 检查是否有 (
		if textEnd >= len(result) || result[textEnd] != '(' {
			start = textEnd + 1
			continue
		}

		// 查找 )
		urlEndIdx := strings.Index(result[textEnd+1:], ")")
		if urlEndIdx == -1 {
			break
		}

		urlEnd := textEnd + urlEndIdx + 2

		// 提取文本和 URL
		text := result[pos+1 : textEnd]
		_ = result[textEnd+2 : urlEnd-1] // URL 提取（当前未使用）

		// 替换为样式化版本
		style := lipgloss.NewStyle().
			Underline(true).
			Foreground(r.styles.Info.GetForeground())
		result = result[:pos] + style.Render(text) + result[urlEnd:]

		// 跳过已处理部分
		start = urlEnd
	}

	return result
}

// RenderCodeBlock 渲染代码块
func (r *MarkdownRenderer) RenderCodeBlock(code string) string {
	style := r.styles.CodeBlock
	return style.Render(code)
}

// RenderQuote 渲染引用
func (r *MarkdownRenderer) RenderQuote(text string) string {
	style := lipgloss.NewStyle().
		Foreground(r.styles.Info.GetForeground()).
		BorderLeft(true).
		BorderForeground(r.styles.Info.GetForeground()).
		PaddingLeft(1).
		MarginLeft(1)

	return style.Render(text)
}
