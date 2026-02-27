package tui

import (
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// Tab 标签页
type Tab struct {
	ID    string
	Title string
	Active bool
}

// TabsModel 标签页组件
type TabsModel struct {
	tabs     []Tab
	active   int
	width    int
	height   int
}

// NewTabsModel 创建标签页组件
func NewTabsModel() TabsModel {
	return TabsModel{
		tabs:   make([]Tab, 0),
		active: 0,
	}
}

// AddTab 添加标签页
func (m *TabsModel) AddTab(id, title string) {
	m.tabs = append(m.tabs, Tab{
		ID:     id,
		Title:  title,
		Active: false,
	})

	// 如果是第一个标签页，设为活跃
	if len(m.tabs) == 1 {
		m.tabs[0].Active = true
	}
}

// RemoveTab 移除标签页
func (m *TabsModel) RemoveTab(id string) bool {
	for i, tab := range m.tabs {
		if tab.ID == id {
			m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)

			// 如果移除的是活跃标签页，切换到相邻的标签页
			if i == m.active {
				if len(m.tabs) > 0 {
					m.active = i
					if m.active >= len(m.tabs) {
						m.active = len(m.tabs) - 1
					}
					m.tabs[m.active].Active = true
				}
			} else if i < m.active {
				m.active--
			}

			return true
		}
	}
	return false
}

// SetActive 设置活跃标签页
func (m *TabsModel) SetActive(id string) bool {
	for i := range m.tabs {
		if m.tabs[i].ID == id {
			// 取消旧的活跃标签页
			if m.active < len(m.tabs) {
				m.tabs[m.active].Active = false
			}
			// 设置新的活跃标签页
			m.active = i
			m.tabs[i].Active = true
			return true
		}
	}
	return false
}

// GetActive 获取活跃标签页
func (m *TabsModel) GetActive() *Tab {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return &m.tabs[m.active]
}

// GetActiveID 获取活跃标签页 ID
func (m *TabsModel) GetActiveID() string {
	tab := m.GetActive()
	if tab == nil {
		return ""
	}
	return tab.ID
}

// Update 更新组件
func (m TabsModel) Update(msg tea.Msg) (TabsModel, tea.Cmd) {
	return m, nil
}

// View 渲染标签页
func (m TabsModel) View() string {
	if len(m.tabs) == 0 {
		return ""
	}

	var tabs []string
	for _, tab := range m.tabs {
		style := tabStyle
		if tab.Active {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(tab.Title))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// SetSize 设置尺寸
func (m *TabsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// UpdateTabTitle 更新标签页标题
func (m *TabsModel) UpdateTabTitle(id, title string) {
	for i := range m.tabs {
		if m.tabs[i].ID == id {
			m.tabs[i].Title = title
			break
		}
	}
}

// GetTabCount 获取标签页数量
func (m *TabsModel) GetTabCount() int {
	return len(m.tabs)
}

// NextTab 切换到下一个标签页
func (m *TabsModel) NextTab() {
	if len(m.tabs) == 0 {
		return
	}

	if m.active >= 0 && m.active < len(m.tabs) {
		m.tabs[m.active].Active = false
	}

	m.active = (m.active + 1) % len(m.tabs)
	m.tabs[m.active].Active = true
}

// PrevTab 切换到上一个标签页
func (m *TabsModel) PrevTab() {
	if len(m.tabs) == 0 {
		return
	}

	if m.active >= 0 && m.active < len(m.tabs) {
		m.tabs[m.active].Active = false
	}

	m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
	m.tabs[m.active].Active = true
}

// 标签页样式
var (
	tabStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 2).
		Border(lipgloss.Border{Left: " ", Right: " "})

	activeTabStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 2).
		Border(lipgloss.Border{Left: " ", Right: " "})
)
