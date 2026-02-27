package tui

import "github.com/charmbracelet/lipgloss"

var (
	// 标题样式
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		PaddingLeft(2).
		PaddingRight(2)

	// 副标题样式
	subtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		PaddingLeft(2).
		PaddingRight(2)

	// 输入框样式
	inputStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	// 状态栏样式
	statusBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("248")).
		Padding(0, 1)

	// 日志头部样式
	logHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1).
		MarginBottom(1)

	// 成功状态样式
	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)

	// 错误状态样式
	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true)

	// 信息样式
	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("12"))

	// 命令列表样式
	commandStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("14"))

	// 内容区域样式
	contentStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingRight(2)
)
