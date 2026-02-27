package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View 渲染 TUI 界面
func (m Model) View() string {
	if !m.ready {
		return "正在初始化..."
	}

	var sections []string

	// 1. 标题栏
	sections = append(sections, m.renderHeader())

	// 2. 内容区域（根据模式不同）
	if m.mode == ModeLogView {
		sections = append(sections, m.renderLogView())
	} else {
		sections = append(sections, m.renderNormalView())
	}

	// 3. 输入栏
	sections = append(sections, m.renderInput())

	// 4. 状态栏
	sections = append(sections, m.renderStatusBar())

	return strings.Join(sections, "\n")
}

// renderHeader 渲染标题栏
func (m Model) renderHeader() string {
	styles := GetStyles()
	title := styles.Title.Render("Linkyun Edge Proxy")
	agentUUID := m.config.AgentUUID
	if len(agentUUID) > 20 {
		agentUUID = agentUUID[:20] + "..."
	}
	info := styles.Subtitle.Render(fmt.Sprintf("Agent: %s | Server: %s", agentUUID, m.config.ServerURL))
	return lipgloss.JoinVertical(lipgloss.Left, title, info)
}

// renderNormalView 渲染普通视图
func (m Model) renderNormalView() string {
	styles := GetStyles()
	var lines []string

	lines = append(lines, "")
	lines = append(lines, styles.Content.Render(fmt.Sprintf("运行状态: %s", m.getStatusText())))
	lines = append(lines, styles.Content.Render(fmt.Sprintf("运行时间: %s", m.getUptime())))
	lines = append(lines, styles.Content.Render(fmt.Sprintf("处理请求: %d (成功: %d, 失败: %d)",
		m.stats.RequestCount, m.stats.SuccessCount, m.stats.ErrorCount)))
	if m.stats.AvgResponseTime > 0 {
		lines = append(lines, styles.Content.Render(fmt.Sprintf("平均响应时间: %s", m.stats.AvgResponseTime.Round(time.Millisecond))))
	}
	lines = append(lines, "")
	lines = append(lines, styles.Content.Render("可用命令:"))
	lines = append(lines, styles.Content.Render(styles.Command.Render("  /status")+styles.Info.Render(" - 查看实时日志")))
	lines = append(lines, styles.Content.Render(styles.Command.Render("  /clear")+styles.Info.Render("  - 退出日志查看")))
	lines = append(lines, styles.Content.Render(styles.Command.Render("  /config")+styles.Info.Render(" - 显示配置信息")))
	lines = append(lines, styles.Content.Render(styles.Command.Render("  /stats")+styles.Info.Render("  - 显示详细统计")))
	lines = append(lines, styles.Content.Render(styles.Command.Render("  /help")+styles.Info.Render("   - 显示帮助")))
	lines = append(lines, styles.Content.Render(styles.Command.Render("  /quit")+styles.Info.Render("   - 退出程序")))

	return strings.Join(lines, "\n")
}

// renderLogView 渲染日志视图
func (m Model) renderLogView() string {
	styles := GetStyles()
	header := styles.Header.Render("实时日志 (输入 /clear 退出，使用方向键滚动，可鼠标选择复制)")

	// 将日志转换为纯文本（可被鼠标选择）
	var logLines []string
	for _, log := range m.logs {
		timestamp := log.Timestamp.Format("15:04:05")
		var levelPrefix string
		switch log.Level {
		case 1: // INFO
			levelPrefix = "[INFO] "
		case 2: // WARN
			levelPrefix = "[WARN] "
		case 3: // ERROR
			levelPrefix = "[ERROR] "
		case 0: // DEBUG
			levelPrefix = "[DEBUG] "
		default:
			levelPrefix = "[UNKNOWN] "
		}

		// 直接渲染为纯文本
		logLines = append(logLines, timestamp+" "+levelPrefix+log.Message)
	}

	// 根据滚动偏移量显示部分日志（从最新开始）
	availableHeight := m.height - 5 // 减去头部和状态栏的高度
	visibleLines := calculateVisibleLines(logLines, m.logScroll, availableHeight)

	// 直接返回日志内容作为纯文本（不带 lipgloss 样式）
	if len(visibleLines) == 0 {
		visibleLines = []string{"暂无日志"}
	}

	logContent := "\n" + joinStrings(visibleLines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left, header, logContent)
}

// calculateVisibleLines 根据滚动偏移量和可用高度计算可见的日志行
func calculateVisibleLines(logLines []string, scrollFromEnd, height int) []string {
	if len(logLines) == 0 {
		return logLines
	}

	// 确保滚动偏移量在有效范围内
	if scrollFromEnd < 0 {
		scrollFromEnd = 0
	}
	if scrollFromEnd > len(logLines) {
		scrollFromEnd = len(logLines)
	}

	// 从末尾开始，向前滚动
	startIndex := len(logLines) - scrollFromEnd - height
	if startIndex < 0 {
		startIndex = 0
	}

	// 计算结束索引
	endIndex := len(logLines) - scrollFromEnd
	if endIndex > len(logLines) {
		endIndex = len(logLines)
	}

	return logLines[startIndex:endIndex]
}

// joinStrings 连接字符串数组
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// renderInput 渲染输入框
func (m Model) renderInput() string {
	styles := GetStyles()
	return styles.Header.Render("> " + m.textInput.View())
}

// renderStatusBar 渲染状态栏
func (m Model) renderStatusBar() string {
	styles := GetStyles()
	status := fmt.Sprintf("模式: %s | 日志: %d 条 | Ctrl+C 退出",
		m.getModeText(),
		len(m.logs),
	)
	return styles.Footer.Width(m.width).Render(status)
}

// getUptime 获取运行时间
func (m Model) getUptime() string {
	d := time.Since(m.startTime)
	return fmt.Sprintf("%02d:%02d:%02d",
		int(d.Hours()),
		int(d.Minutes())%60,
		int(d.Seconds())%60,
	)
}

// getStatusText 获取状态文本
func (m Model) getStatusText() string {
	styles := GetStyles()
	if m.stats.Connected {
		return styles.Success.Render("✓ 已连接")
	}
	return styles.Error.Render("✗ 未连接")
}

// getModeText 获取模式文本
func (m Model) getModeText() string {
	if m.mode == ModeLogView {
		return "日志查看"
	}
	return "命令模式"
}
