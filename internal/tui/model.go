package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"linkyun-edge-proxy/internal/config"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/proxy"
)

// ViewMode 视图模式
type ViewMode int

const (
	ModeNormal  ViewMode = iota // 普通命令模式
	ModeLogView                 // 日志滚动模式
)

// Model Bubble Tea Model
type Model struct {
	// 组件
	textInput textinput.Model
	viewport  viewport.Model

	// 状态
	mode      ViewMode
	logs      []logger.LogEntry
	stats     proxy.ProxyStats
	config    *config.Config
	startTime time.Time

	// 通道
	logChan   <-chan logger.LogEntry
	statsChan <-chan proxy.ProxyStats

	// UI 状态
	ready       bool
	width       int
	height      int
	quit        bool
	logScroll   int // 日志滚动偏移量（从末尾开始）
}

// NewModel 创建新的 Model
func NewModel(logChan <-chan logger.LogEntry, statsChan <-chan proxy.ProxyStats, cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "输入命令 (/help 查看帮助)"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	return Model{
		textInput: ti,
		logChan:   logChan,
		statsChan: statsChan,
		config:    cfg,
		mode:      ModeNormal,
		startTime: time.Now(),
		logs:      make([]logger.LogEntry, 0, 1000),
	}
}

// IsQuit 返回是否用户主动退出
func (m Model) IsQuit() bool {
	return m.quit
}
