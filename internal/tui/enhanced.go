package tui

import (
	"context"
	"fmt"
	"linkyun-edge-proxy/internal/chat"
	"linkyun-edge-proxy/internal/commands"
	"linkyun-edge-proxy/internal/config"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/proxy"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// localCompletionDoneMsg 本地 LLM 完成后的消息（用于 TUI 聊天测试）
type localCompletionDoneMsg struct {
	resp *proxy.EdgeResponse
	err  error
}

// 复用 update.go 中的消息类型（避免重复声明）
// type logMsg logger.LogEntry
// type statsMsg proxy.ProxyStats
// type tickMsg time.Time

// EnhancedViewMode 增强视图模式
const (
	EnhancedModeChat EnhancedViewMode = iota
	EnhancedModeLogs
)

// EnhancedViewMode 视图模式
type EnhancedViewMode int

// EnhancedModel 增强型 TUI 模型
type EnhancedModel struct {
	// 基础组件
	textInput textinput.Model
	logChan   <-chan logger.LogEntry
	statsChan <-chan proxy.ProxyStats
	config    *config.Config
	startTime time.Time

	// 视图模式
	viewMode EnhancedViewMode

	// 状态
	logs      []logger.LogEntry
	stats     proxy.ProxyStats
	ready     bool
	width     int
	height    int
	quit      bool

	// 会话相关
	chatManager      *chat.Manager
	commandRegistry  *commands.Registry
	conversation     []string              // 展示用：You/AI 文本行
	messageHistory   []proxy.EdgeMessage   // 发给 LLM 的 user/assistant 历史
	proxy            *proxy.Proxy          // 非 nil 时用于本地测试聊天（CompleteLocal）
}

// NewEnhancedModel 创建增强型模型。proxy 可选：非 nil 时可在聊天界面输入消息本地测试 LLM + Skill。
func NewEnhancedModel(logChan <-chan logger.LogEntry, statsChan <-chan proxy.ProxyStats, cfg *config.Config,
	chatManager *chat.Manager, commandRegistry *commands.Registry, p *proxy.Proxy) *EnhancedModel {

	ti := textinput.New()
	ti.Placeholder = "输入命令或消息 (/help 查看帮助)"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	model := &EnhancedModel{
		textInput:       ti,
		logChan:         logChan,
		statsChan:       statsChan,
		config:          cfg,
		viewMode:        EnhancedModeChat,
		startTime:       time.Now(),
		logs:            make([]logger.LogEntry, 0, 1000),
		chatManager:     chatManager,
		commandRegistry: commandRegistry,
		conversation:    make([]string, 0),
		messageHistory:  make([]proxy.EdgeMessage, 0),
		proxy:           p,
	}

	return model
}

// Init 实现 tea.Model 接口
func (m *EnhancedModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		waitForLog(m.logChan),
		waitForStats(m.statsChan),
		tickEvery(time.Second),
	)
}

// Update 实现 tea.Model 接口
func (m *EnhancedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// 处理命令或消息
			return m.handleInput()
		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit
		case tea.KeyTab:
			// 用 Tab 键切换模式
			switch m.viewMode {
			case EnhancedModeChat:
				m.viewMode = EnhancedModeLogs
			case EnhancedModeLogs:
				m.viewMode = EnhancedModeChat
			}
			return m, nil
		}

		// 更新 textinput
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case logMsg:
		// 接收新日志
		m.logs = append(m.logs, logger.LogEntry(msg))
		if len(m.logs) > 10000 {
			m.logs = m.logs[1:]
		}
		cmds = append(cmds, waitForLog(m.logChan))

	case statsMsg:
		// 更新统计信息
		m.stats = proxy.ProxyStats(msg)
		cmds = append(cmds, waitForStats(m.statsChan))

	case tickMsg:
		// 定期刷新
		cmds = append(cmds, tickEvery(time.Second))

	case localCompletionDoneMsg:
		if msg.err != nil {
			m.conversation = append(m.conversation, fmt.Sprintf("AI: [错误] %v", msg.err))
		} else if msg.resp != nil {
			content := msg.resp.Content
			if !msg.resp.Success && msg.resp.Error != "" {
				content = "[错误] " + msg.resp.Error
			}
			m.conversation = append(m.conversation, "AI: "+content)
			m.messageHistory = append(m.messageHistory, proxy.EdgeMessage{Role: "assistant", Content: content})
		}
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// View 实现 tea.Model 接口
func (m *EnhancedModel) View() string {
	if !m.ready {
		return "正在初始化..."
	}

	var sections []string

	// 1. 标题栏
	sections = append(sections, m.renderHeader())

	// 2. 内容区域
	switch m.viewMode {
	case EnhancedModeChat:
		sections = append(sections, m.renderChatView())
	case EnhancedModeLogs:
		sections = append(sections, m.renderLogView())
	}

	// 3. 输入栏（只在聊天模式下显示）
	if m.viewMode == EnhancedModeChat {
		sections = append(sections, m.renderInput())
	}

	// 4. 状态栏
	sections = append(sections, m.renderStatusBar())

	return strings.Join(sections, "\n")
}

// handleInput 处理输入
func (m *EnhancedModel) handleInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textInput.Value())

	// 清空输入
	m.textInput.SetValue("")

	if input == "" {
		return m, nil
	}

	// 检查是否是内置快捷命令
	switch input {
	case "/status":
		// 显示连接状态
		statusMsg := ""
		if m.stats.Connected {
			statusMsg = "✓ 已连接到服务器"
		} else {
			statusMsg = "✗ 未连接到服务器"
		}
		m.conversation = append(m.conversation, fmt.Sprintf("[状态] %s", statusMsg))
		return m, nil
	}

	// 检查是否是其他命令（以 / 开头）
	if strings.HasPrefix(input, "/") {
		output, err := m.commandRegistry.Execute(&commands.Context{
			Manager: m.chatManager,
		}, input)

		if err != nil {
			// 显示错误信息到对话
			m.conversation = append(m.conversation, fmt.Sprintf("[ERROR] %v", err))
			return m, nil
		}

		// 显示命令输出到对话
		if output != "" {
			m.conversation = append(m.conversation, output)
		}
		return m, nil
	}

	// 非命令，作为对话消息处理
	m.conversation = append(m.conversation, fmt.Sprintf("You: %s", input))
	m.messageHistory = append(m.messageHistory, proxy.EdgeMessage{Role: "user", Content: input})

	if m.proxy == nil {
		m.conversation = append(m.conversation, "AI: [本地测试未启用：未传入 Proxy]")
		return m, nil
	}

	// 构建本地请求并异步调用 CompleteLocal
	req := &proxy.EdgeRequest{
		RequestID:      fmt.Sprintf("local-%d", time.Now().UnixNano()),
		AgentUUID:      m.config.AgentUUID,
		Type:           "chat",
		Messages:       make([]proxy.EdgeMessage, len(m.messageHistory)),
		Temperature:    0.7,
		MaxTokens:      4096,
		MemoryEnabled:  false,
		Timestamp:      time.Now(),
	}
	copy(req.Messages, m.messageHistory)

	if m.config.LLM.Temperature > 0 {
		req.Temperature = m.config.LLM.Temperature
	}
	if m.config.LLM.MaxTokens > 0 {
		req.MaxTokens = m.config.LLM.MaxTokens
	}

	cmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		resp, err := m.proxy.CompleteLocal(ctx, req)
		return localCompletionDoneMsg{resp: resp, err: err}
	}
	return m, cmd
}

// renderHeader 渲染标题栏
func (m *EnhancedModel) renderHeader() string {
	styles := GetStyles()
	title := styles.Title.Render("Linkyun Edge Proxy")
	modeText := ""
	switch m.viewMode {
	case EnhancedModeChat:
		modeText = "CLI 模式"
	case EnhancedModeLogs:
		modeText = "日志模式"
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", styles.Subtitle.Render(modeText))
}

// renderChatView 渲染聊天视图
func (m *EnhancedModel) renderChatView() string {
	styles := GetStyles()
	if len(m.conversation) == 0 {
		return styles.Info.Render("\n  暂无对话记录，输入消息开始聊天...")
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, styles.Content.Render("快捷键:"))
	lines = append(lines, styles.Content.Render("  按 Enter - 执行命令/发送消息"))
	lines = append(lines, styles.Content.Render("  按 Tab - 切换到日志模式"))
	lines = append(lines, styles.Content.Render("  按 Ctrl+C - 退出"))
	lines = append(lines, "")
	lines = append(lines, styles.Content.Render("对话（可输入消息本地测试 LLM + Skill）:"))
	for _, msg := range lastN(m.conversation, 25) {
		lines = append(lines, styles.Content.Render("  "+msg))
	}
	return strings.Join(lines, "\n")
}

// renderLogView 渲染日志视图
func (m *EnhancedModel) renderLogView() string {
	styles := GetStyles()
	if len(m.logs) == 0 {
		return styles.Info.Render("\n  暂无日志...")
	}

	var sb strings.Builder
	sb.WriteString("\n")
	logCount := len(m.logs)
	start := 0
	if logCount > 20 {
		start = logCount - 20
	}
	for i := start; i < logCount; i++ {
		entry := m.logs[i]
		levelStr := "INFO"
		switch entry.Level {
		case logger.DEBUG:
			levelStr = "DEBUG"
		case logger.WARN:
			levelStr = "WARN"
		case logger.ERROR:
			levelStr = "ERROR"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			entry.Timestamp.Format("15:04:05"), levelStr, entry.Message))
	}
	return sb.String()
}

// renderInput 渲染输入框
func (m *EnhancedModel) renderInput() string {
	styles := GetStyles()
	prompt := "> "
	if m.viewMode == EnhancedModeChat {
		prompt = "聊天: "
	}
	return styles.Header.Render(prompt + m.textInput.View())
}

// renderStatusBar 渲染状态栏
func (m *EnhancedModel) renderStatusBar() string {
	styles := GetStyles()

	// 连接状态
	connStatus := "未连接"
	if m.stats.Connected {
		connStatus = "已连接"
	}

	var status string
	if m.viewMode == EnhancedModeLogs {
		// 日志模式不显示对话数
		status = fmt.Sprintf("连接: %s | 日志: %d | Tab=CLI | Ctrl+C=退出",
			connStatus, len(m.logs))
	} else {
		// CLI 模式
		status = fmt.Sprintf("连接: %s | 日志: %d | 对话: %d | Tab=日志 | Ctrl+C=退出",
			connStatus, len(m.logs), len(m.conversation))
	}
	return styles.Footer.Width(m.width).Render(status)
}

// getUptime 获取运行时间
func (m *EnhancedModel) getUptime() string {
	d := time.Since(m.startTime)
	return fmt.Sprintf("%02d:%02d:%02d",
		int(d.Hours()),
		int(d.Minutes())%60,
		int(d.Seconds())%60,
	)
}

// IsQuit 返回是否用户主动退出
func (m *EnhancedModel) IsQuit() bool {
	return m.quit
}

// lastN 获取最后 N 个元素
func lastN(slice []string, n int) []string {
	if len(slice) <= n {
		return slice
	}
	return slice[len(slice)-n:]
}

