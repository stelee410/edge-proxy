package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbles/list"
)

// FileInfo 文件信息
type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	ModTime  time.Time
	Mode     os.FileMode
}

// FileBrowserModel 文件浏览器模型
type FileBrowserModel struct {
	currentPath string
	files       []FileInfo
	selected    int
	list        list.Model
	width       int
	height      int
	styles      *ThemeStyles
	showHidden  bool
}

// NewFileBrowserModel 创建文件浏览器模型
func NewFileBrowserModel() FileBrowserModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "File Browser"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return FileBrowserModel{
		currentPath: ".",
		files:       make([]FileInfo, 0),
		selected:    0,
		list:        l,
		styles:      &GlobalTheme.Styles,
		showHidden:  false,
	}
}

// Init 初始化
func (m FileBrowserModel) Init() tea.Cmd {
	return nil
}

// Update 更新模型
func (m FileBrowserModel) Update(msg tea.Msg) (FileBrowserModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown:
			if m.selected < len(m.files)-1 {
				m.selected++
			}
		case tea.KeyEnter:
			if len(m.files) > 0 && m.files[m.selected].IsDir {
				m.NavigateTo(m.files[m.selected].Path)
			}
		case tea.KeyBackspace:
			m.NavigateUp()
		case tea.KeyLeft:
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyRight:
			if m.selected < len(m.files)-1 {
				m.selected++
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View 渲染视图
func (m FileBrowserModel) View() string {
	if len(m.files) == 0 {
		return m.styles.Content.Render("No files found")
	}

	// 渲染文件列表
	var sb strings.Builder

	// 路径栏
	pathStyle := m.styles.Info
	sb.WriteString(pathStyle.Render("Path: " + m.currentPath))
	sb.WriteString("\n\n")

	// 文件列表
	for i, file := range m.files {
		// 跳过隐藏文件（除非显示）
		if !m.showHidden && strings.HasPrefix(file.Name, ".") {
			continue
		}

		// 选中项样式
		if i == m.selected {
			style := lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Foreground(lipgloss.Color("255")).
				Padding(0, 1)

			icon := "📁"
			if !file.IsDir {
				icon = getFileIcon(file.Name)
			}

			sb.WriteString(style.Render(icon + " " + file.Name))
		} else {
			style := m.styles.Content

			icon := "📁"
			if !file.IsDir {
				icon = getFileIcon(file.Name)
			}

			sb.WriteString(style.Render(icon + " " + file.Name))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Refresh 刷新文件列表
func (m *FileBrowserModel) Refresh() error {
	return m.loadDirectory(m.currentPath)
}

// NavigateTo 导航到指定路径
func (m *FileBrowserModel) NavigateTo(path string) error {
	if path == "" {
		return nil
	}

	// 规范化路径
	path = filepath.Clean(path)

	// 检查是否是目录
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		// 返回文件信息
		return nil
	}

	return m.loadDirectory(path)
}

// NavigateUp 导航到上级目录
func (m *FileBrowserModel) NavigateUp() {
	parent := filepath.Dir(m.currentPath)
	if parent != m.currentPath {
		m.NavigateTo(parent)
	}
}

// loadDirectory 加载目录
func (m *FileBrowserModel) loadDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	m.currentPath = path
	m.files = make([]FileInfo, 0, len(entries))

	// 添加上级目录
	if path != "/" && path != "." {
		m.files = append(m.files, FileInfo{
			Name:    "..",
			Path:    filepath.Dir(path),
			IsDir:   true,
			ModTime: time.Now(),
			Mode:    os.ModeDir,
		})
	}

	// 添加目录和文件
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(path, entry.Name())
		m.files = append(m.files, FileInfo{
			Name:    entry.Name(),
			Path:    filePath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode(),
		})
	}

	// 排序：目录优先，然后按名称排序
	sort.Slice(m.files, func(i, j int) bool {
		if m.files[i].IsDir != m.files[j].IsDir {
			return m.files[i].IsDir
		}
		return strings.ToLower(m.files[i].Name) < strings.ToLower(m.files[j].Name)
	})

	m.selected = 0
	return nil
}

// GetSelected 获取选中项
func (m FileBrowserModel) GetSelected() *FileInfo {
	if len(m.files) == 0 {
		return nil
	}
	return &m.files[m.selected]
}

// GetCurrentPath 获取当前路径
func (m FileBrowserModel) GetCurrentPath() string {
	return m.currentPath
}

// SetSize 设置尺寸
func (m *FileBrowserModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height-2)
}

// ToggleHidden 切换显示隐藏文件
func (m *FileBrowserModel) ToggleHidden() {
	m.showHidden = !m.showHidden
}

// getFileIcon 获取文件图标
func getFileIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	icons := map[string]string{
		".go":    "🐹",
		".js":    "📜",
		".ts":    "📘",
		".py":    "🐍",
		".rb":    "💎",
		".java":  "☕",
		".c":     "🔧",
		".cpp":   "⚙️",
		".h":     "📝",
		".rs":    "🦀",
		".sh":    "🔘",
		".bash":  "🔘",
		".yaml":  "📋",
		".yml":   "📋",
		".json":  "📄",
		".xml":   "📄",
		".html":  "🌐",
		".htm":   "🌐",
		".css":   "🎨",
		".scss":  "🎨",
		".sql":   "🗄️",
		".md":    "📝",
		".txt":   "📄",
		".pdf":   "📕",
		".doc":   "📘",
		".docx":  "📘",
		".xls":   "📗",
		".xlsx":  "📗",
		".ppt":   "📙",
		".pptx":  "📙",
		".zip":   "📦",
		".tar":   "📦",
		".gz":    "📦",
		".rar":   "📦",
		".7z":    "📦",
		".exe":   "⚙️",
		".dll":   "⚙️",
		".so":    "⚙️",
		".dylib": "⚙️",
		".png":   "🖼️",
		".jpg":   "🖼️",
		".jpeg":  "🖼️",
		".gif":   "🖼️",
		".svg":   "🖼️",
		".mp3":   "🎵",
		".mp4":   "🎬",
		".wav":   "🎵",
		".avi":   "🎬",
		".mkv":   "🎬",
	}

	if icon, ok := icons[ext]; ok {
		return icon
	}

	return "📄"
}

// ReadFile 读取文件内容
func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// WriteFile 写入文件
func WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// GetFileExtension 获取文件扩展名
func GetFileExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
