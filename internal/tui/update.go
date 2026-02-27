package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/proxy"
)

// 自定义消息类型
type logMsg logger.LogEntry
type statsMsg proxy.ProxyStats
type tickMsg time.Time

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		waitForLog(m.logChan),
		waitForStats(m.statsChan),
		tickEvery(time.Second),
	)
}

// Update 处理消息并更新状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 日志模式下处理滚动键
		if m.mode == ModeLogView {
			switch msg.Type {
			case tea.KeyUp, tea.KeyCtrlP:
				m.logScroll += 5
				if m.logScroll > len(m.logs) {
					m.logScroll = len(m.logs)
				}
			case tea.KeyDown, tea.KeyCtrlN:
				m.logScroll -= 5
				if m.logScroll < 0 {
					m.logScroll = 0
				}
			case tea.KeyEnter:
				// 在日志模式下回车退出到命令模式
				m.mode = ModeNormal
				m.logScroll = 0
			}
			return m, nil
		}

		// 普通命令模式
		switch msg.Type {
		case tea.KeyEnter:
			// 处理命令
			return m.handleCommand()
		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// 保持 viewport 用于其他可能的用途
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-5)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 5
		}

	case logMsg:
		// 接收新日志
		m.logs = append(m.logs, logger.LogEntry(msg))
		// 限制日志数量，避免内存无限增长
		if len(m.logs) > 10000 {
			m.logs = m.logs[1:]
		}
		// 在日志模式下，有新日志时重置滚动位置到顶部（显示最新）
		if m.mode == ModeLogView {
			m.logScroll = 0
		}
		cmds = append(cmds, waitForLog(m.logChan))

	case statsMsg:
		// 更新统计信息
		m.stats = proxy.ProxyStats(msg)
		cmds = append(cmds, waitForStats(m.statsChan))

	case tickMsg:
		// 定期刷新（用于更新运行时间）
		cmds = append(cmds, tickEvery(time.Second))
	}

	// 更新输入框（仅在非日志模式下）
	if m.mode != ModeLogView {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// waitForLog 等待日志消息
func waitForLog(ch <-chan logger.LogEntry) tea.Cmd {
	return func() tea.Msg {
		return logMsg(<-ch)
	}
}

// waitForStats 等待统计消息
func waitForStats(ch <-chan proxy.ProxyStats) tea.Cmd {
	return func() tea.Msg {
		return statsMsg(<-ch)
	}
}

// tickEvery 定时触发
func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
