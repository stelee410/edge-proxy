package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ThemeType 主题类型
type ThemeType string

const (
	ThemeDark  ThemeType = "dark"
	ThemeLight ThemeType = "light"
	ThemeAuto  ThemeType = "auto"
)

// Theme 主题定义
type Theme struct {
	Name   ThemeType
	Colors ThemeColors
	Styles ThemeStyles
}

// ThemeColors 颜色定义
type ThemeColors struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Success   lipgloss.Color
	Info      lipgloss.Color
	FG        lipgloss.Color // 前景色
	BG        lipgloss.Color // 背景色
	Muted     lipgloss.Color
	Border    lipgloss.Color
}

// ThemeStyles 样式定义
type ThemeStyles struct {
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Content     lipgloss.Style
	Info        lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Command     lipgloss.Style
	UserMsg     lipgloss.Style
	AssistantMsg lipgloss.Style
	SystemMsg   lipgloss.Style
	CodeBlock   lipgloss.Style
	Header      lipgloss.Style
	Footer      lipgloss.Style
	LogContent  lipgloss.Style // 日志内容样式（纯文本，可被鼠标选择）
}

// GetTheme 获取主题
func GetTheme(themeType ThemeType) *Theme {
	switch themeType {
	case ThemeLight:
		return lightTheme()
	case ThemeDark:
		return darkTheme()
	default:
		return darkTheme()
	}
}

// darkTheme 暗色主题
func darkTheme() *Theme {
	return &Theme{
		Name: ThemeDark,
		Colors: ThemeColors{
			Primary:   lipgloss.Color("229"), // 黄色
			Secondary: lipgloss.Color("147"), // 浅蓝
			Accent:    lipgloss.Color("57"),  // 紫色
			Error:     lipgloss.Color("196"), // 红色
			Warning:   lipgloss.Color("228"), // 橙色
			Success:   lipgloss.Color("46"),  // 绿色
			Info:      lipgloss.Color("39"),  // 蓝色
			FG:        lipgloss.Color("252"), // 白色
			BG:        lipgloss.Color("235"), // 深灰
			Muted:     lipgloss.Color("245"), // 浅灰
			Border:    lipgloss.Color("240"), // 边框色
		},
		Styles: ThemeStyles{
			Title: lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Bold(true).
				Padding(0, 1),
			Subtitle: lipgloss.NewStyle().
				Foreground(lipgloss.Color("147")).
				Padding(0, 1),
			Content: lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Padding(0, 1),
			Info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Padding(0, 1),
			Error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Padding(0, 1),
			Success: lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Padding(0, 1),
			Warning: lipgloss.NewStyle().
				Foreground(lipgloss.Color("228")).
				Padding(0, 1),
			Command: lipgloss.NewStyle().
				Foreground(lipgloss.Color("57")).
				Bold(true),
			UserMsg: lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Padding(0, 1),
			AssistantMsg: lipgloss.NewStyle().
				Foreground(lipgloss.Color("147")).
				Padding(0, 1),
			SystemMsg: lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 1).
				Italic(true),
			CodeBlock: lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("230")).
				Padding(1).
				Border(lipgloss.RoundedBorder()),
			Header: lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Foreground(lipgloss.Color("252")).
				Bold(true).
				Padding(0, 1),
			Footer: lipgloss.NewStyle().
				Background(lipgloss.Color("235")).
				Foreground(lipgloss.Color("245")).
				Padding(0, 1),
			LogContent: lipgloss.NewStyle().
				// 纯文本样式，不带任何颜色，便于鼠标选择和复制
				Padding(0, 1),
		},
	}
}

// lightTheme 亮色主题
func lightTheme() *Theme {
	return &Theme{
		Name: ThemeLight,
		Colors: ThemeColors{
			Primary:   lipgloss.Color("94"),  // 蓝色
			Secondary: lipgloss.Color("30"),  // 深蓝
			Accent:    lipgloss.Color("128"), // 紫色
			Error:     lipgloss.Color("124"), // 红色
			Warning:   lipgloss.Color("172"), // 橙色
			Success:   lipgloss.Color("28"),  // 绿色
			Info:      lipgloss.Color("25"),  // 青色
			FG:        lipgloss.Color("234"), // 黑色
			BG:        lipgloss.Color("255"), // 白色
			Muted:     lipgloss.Color("244"), // 灰色
			Border:    lipgloss.Color("241"), // 边框色
		},
		Styles: ThemeStyles{
			Title: lipgloss.NewStyle().
				Foreground(lipgloss.Color("94")).
				Bold(true).
				Padding(0, 1),
			Subtitle: lipgloss.NewStyle().
				Foreground(lipgloss.Color("30")).
				Padding(0, 1),
			Content: lipgloss.NewStyle().
				Foreground(lipgloss.Color("234")).
				Padding(0, 1),
			Info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("25")).
				Padding(0, 1),
			Error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("124")).
				Padding(0, 1),
			Success: lipgloss.NewStyle().
				Foreground(lipgloss.Color("28")).
				Padding(0, 1),
			Warning: lipgloss.NewStyle().
				Foreground(lipgloss.Color("172")).
				Padding(0, 1),
			Command: lipgloss.NewStyle().
				Foreground(lipgloss.Color("128")).
				Bold(true),
			UserMsg: lipgloss.NewStyle().
				Foreground(lipgloss.Color("234")).
				Padding(0, 1),
			AssistantMsg: lipgloss.NewStyle().
				Foreground(lipgloss.Color("30")).
				Padding(0, 1),
			SystemMsg: lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Padding(0, 1).
				Italic(true),
			CodeBlock: lipgloss.NewStyle().
				Background(lipgloss.Color("252")).
				Foreground(lipgloss.Color("236")).
				Padding(1).
				Border(lipgloss.RoundedBorder()),
			Header: lipgloss.NewStyle().
				Background(lipgloss.Color("94")).
				Foreground(lipgloss.Color("255")).
				Bold(true).
				Padding(0, 1),
			Footer: lipgloss.NewStyle().
				Background(lipgloss.Color("251")).
				Foreground(lipgloss.Color("244")).
				Padding(0, 1),
			LogContent: lipgloss.NewStyle().
				// 纯文本样式，不带任何颜色，便于鼠标选择和复制
				Padding(0, 1),
		},
	}
}

// GlobalTheme 全局主题
var GlobalTheme *Theme = darkTheme()

// SetTheme 设置主题
func SetTheme(themeType ThemeType) {
	GlobalTheme = GetTheme(themeType)
}

// GetStyles 获取当前样式集
func GetStyles() *ThemeStyles {
	return &GlobalTheme.Styles
}

// GetColors 获取当前颜色集
func GetColors() *ThemeColors {
	return &GlobalTheme.Colors
}
