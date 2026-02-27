package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleCommand 处理用户输入的命令
func (m Model) handleCommand() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textInput.Value())
	m.textInput.SetValue("")

	if input == "" {
		return m, nil
	}

	if !strings.HasPrefix(input, "/") {
		// 非命令，显示提示
		return m, nil
	}

	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/status":
		m.mode = ModeLogView
		m.logScroll = 0 // 重置滚动位置
		return m, nil

	case "/clear":
		m.mode = ModeNormal
		return m, nil

	case "/quit", "/exit":
		m.quit = true
		return m, tea.Quit

	case "/help":
		// 显示帮助信息（保持在普通模式）
		m.mode = ModeNormal
		return m, nil

	case "/config":
		// 显示配置信息
		m.mode = ModeNormal
		return m, nil

	case "/stats":
		// 显示统计信息
		m.mode = ModeNormal
		return m, nil

	default:
		// 未知命令
		return m, nil
	}
}
