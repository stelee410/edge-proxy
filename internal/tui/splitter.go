package tui

import (
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// SplitDirection 分割方向
type SplitDirection int

const (
	SplitHorizontal SplitDirection = iota // 水平分割（左右）
	SplitVertical                         // 垂直分割（上下）
)

// SplitPane 分割窗格
type SplitPane struct {
	ID       string
	Title    string
	Content  string
	Weight   float64 // 权重（0-1）
	MinSize  int     // 最小尺寸
	Resizable bool
}

// SplitterModel 分割器模型
type SplitterModel struct {
	panes      []*SplitPane
	direction  SplitDirection
	width      int
	height     int
	gap        int // 分隔符宽度
	separator  string
}

// NewSplitterModel 创建分割器
func NewSplitterModel(direction SplitDirection) *SplitterModel {
	return &SplitterModel{
		panes:     make([]*SplitPane, 0),
		direction: direction,
		gap:       1,
		separator: "│",
	}
}

// AddPane 添加窗格
func (m *SplitterModel) AddPane(id, title string, weight float64) {
	pane := &SplitPane{
		ID:       id,
		Title:    title,
		Content:  "",
		Weight:   weight,
		MinSize:  10,
		Resizable: true,
	}
	m.panes = append(m.panes, pane)
}

// RemovePane 移除窗格
func (m *SplitterModel) RemovePane(id string) bool {
	for i, pane := range m.panes {
		if pane.ID == id {
			m.panes = append(m.panes[:i], m.panes[i+1:]...)
			return true
		}
	}
	return false
}

// GetPane 获取窗格
func (m *SplitterModel) GetPane(id string) *SplitPane {
	for _, pane := range m.panes {
		if pane.ID == id {
			return pane
		}
	}
	return nil
}

// UpdatePaneContent 更新窗格内容
func (m *SplitterModel) UpdatePaneContent(id, content string) bool {
	pane := m.GetPane(id)
	if pane == nil {
		return false
	}
	pane.Content = content
	return true
}

// Update 更新组件
func (m SplitterModel) Update(msg tea.Msg) (SplitterModel, tea.Cmd) {
	return m, nil
}

// View 渲染分割器
func (m SplitterModel) View() string {
	if len(m.panes) == 0 {
		return ""
	}

	if m.direction == SplitHorizontal {
		return m.renderHorizontal()
	}
	return m.renderVertical()
}

// renderHorizontal 渲染水平分割
func (m SplitterModel) renderHorizontal() string {
	var panes []string
	totalWeight := 0.0
	for _, pane := range m.panes {
		totalWeight += pane.Weight
	}

	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)

	availableWidth := m.width - (len(m.panes)-1)*m.gap

	for i, pane := range m.panes {
		// 计算窗格宽度
		width := int(float64(availableWidth) * pane.Weight / totalWeight)
		if width < pane.MinSize {
			width = pane.MinSize
		}

		// 渲染窗格
		paneStyle := lipgloss.NewStyle().Width(width).Height(m.height)
		content := paneStyle.Render(pane.Content)

		// 添加标题
		if pane.Title != "" {
			title := titleStyle.Render(pane.Title)
			content = lipgloss.JoinVertical(lipgloss.Left, title, content)
		}

		panes = append(panes, content)

		// 添加分隔符
		if i < len(m.panes)-1 {
			panes = append(panes, separatorStyle.Render(m.separator))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, panes...)
}

// renderVertical 渲染垂直分割
func (m SplitterModel) renderVertical() string {
	var panes []string
	totalWeight := 0.0
	for _, pane := range m.panes {
		totalWeight += pane.Weight
	}

	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)

	availableHeight := m.height - (len(m.panes)-1)*m.gap

	for i, pane := range m.panes {
		// 计算窗格高度
		height := int(float64(availableHeight) * pane.Weight / totalWeight)
		if height < pane.MinSize {
			height = pane.MinSize
		}

		// 渲染窗格
		paneStyle := lipgloss.NewStyle().Width(m.width).Height(height)
		content := paneStyle.Render(pane.Content)

		// 添加标题
		if pane.Title != "" {
			title := titleStyle.Render(pane.Title)
			content = lipgloss.JoinVertical(lipgloss.Left, title, content)
		}

		panes = append(panes, content)

		// 添加分隔符
		if i < len(m.panes)-1 {
			separatorLine := separatorStyle.Render(string(make([]byte, m.width)))
			for j := range separatorLine {
				separatorLine = separatorLine[:j] + "─" + separatorLine[j+1:]
			}
			panes = append(panes, separatorLine)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, panes...)
}

// SetSize 设置尺寸
func (m *SplitterModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetDirection 设置分割方向
func (m *SplitterModel) SetDirection(direction SplitDirection) {
	m.direction = direction
}

// SetPaneWeight 设置窗格权重
func (m *SplitterModel) SetPaneWeight(id string, weight float64) bool {
	pane := m.GetPane(id)
	if pane == nil {
		return false
	}
	pane.Weight = weight
	return true
}
