package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// KeyAction 键盘动作
type KeyAction string

const (
	KeyQuit          KeyAction = "quit"
	KeySubmit        KeyAction = "submit"
	KeyCancel        KeyAction = "cancel"
	KeyUp            KeyAction = "up"
	KeyDown          KeyAction = "down"
	KeyDownHalf      KeyAction = "down_half"
	KeyDownPage      KeyAction = "down_page"
	KeyUpHalf        KeyAction = "up_half"
	KeyUpPage        KeyAction = "up_page"
	KeyLeft          KeyAction = "left"
	KeyRight         KeyAction = "right"
	KeyHome          KeyAction = "home"
	KeyEnd           KeyAction = "end"
	KeyDelete        KeyAction = "delete"
	KeyDeleteLine    KeyAction = "delete_line"
	KeyDeleteWord    KeyAction = "delete_word"
	KeyBackspace     KeyAction = "backspace"
	KeyBackspaceWord KeyAction = "backspace_word"
	KeyTab           KeyAction = "tab"
	KeyShiftTab      KeyAction = "shift_tab"
	KeyNextTab       KeyAction = "next_tab"
	KeyPrevTab       KeyAction = "prev_tab"
	KeyNewTab        KeyAction = "new_tab"
	KeyCloseTab      KeyAction = "close_tab"
	KeySearch        KeyAction = "search"
	KeyHelp          KeyAction = "help"
	KeyRefresh       KeyAction = "refresh"
	KeySave          KeyAction = "save"
	KeyCopy          KeyAction = "copy"
	KeyPaste         KeyAction = "paste"
	KeyCut           KeyAction = "cut"
	KeyUndo          KeyAction = "undo"
	KeyRedo          KeyAction = "redo"
)

// KeyBinding 键盘绑定
type KeyBinding struct {
	Keys     []tea.KeyType // 绑定的按键
	Alt      bool          // Alt 键
	Ctrl     bool          // Ctrl 键
	Shift    bool          // Shift 键
	Action   KeyAction     // 触发的动作
	Category string        // 分类
}

// KeyBindingManager 键盘绑定管理器
type KeyBindingManager struct {
	bindings map[KeyAction][]*KeyBinding
	vimMode  bool
}

// NewKeyBindingManager 创建键盘绑定管理器
func NewKeyBindingManager() *KeyBindingManager {
	m := &KeyBindingManager{
		bindings: make(map[KeyAction][]*KeyBinding),
		vimMode:  false,
	}

	m.setupDefaultBindings()
	return m
}

// setupDefaultBindings 设置默认绑定
func (m *KeyBindingManager) setupDefaultBindings() {
	// 通用绑定
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlC},
		Action: KeyQuit,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyEnter},
		Action: KeySubmit,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyEsc},
		Action: KeyCancel,
	})

	// 导航
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyUp, tea.KeyCtrlP},
		Action: KeyUp,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyDown, tea.KeyCtrlN},
		Action: KeyDown,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyLeft, tea.KeyCtrlB},
		Action: KeyLeft,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyRight, tea.KeyCtrlF},
		Action: KeyRight,
	})

	// 页面导航
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyPgUp},
		Action: KeyUpPage,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyPgDown},
		Action: KeyDownPage,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlU},
		Action: KeyUpHalf,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlD},
		Action: KeyDownHalf,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyHome},
		Action: KeyHome,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyEnd},
		Action: KeyEnd,
	})

	// 标签页
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlRight},
		Action: KeyNextTab,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlLeft},
		Action: KeyPrevTab,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlT},
		Action: KeyNewTab,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlW},
		Action: KeyCloseTab,
	})

	// 编辑
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyDelete},
		Action: KeyDelete,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyBackspace},
		Action: KeyBackspace,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlA},
		Action: KeyHome,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlE},
		Action: KeyEnd,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlK},
		Action: KeyDeleteLine,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlW},
		Action: KeyDeleteWord,
	})

	// 功能键
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyF1},
		Action: KeyHelp,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyF5},
		Action: KeyRefresh,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlS},
		Action: KeySave,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlR},
		Action: KeyRefresh,
	})
}

// AddBinding 添加绑定
func (m *KeyBindingManager) AddBinding(binding *KeyBinding) {
	m.bindings[binding.Action] = append(m.bindings[binding.Action], binding)
}

// RemoveBinding 移除绑定
func (m *KeyBindingManager) RemoveBinding(action KeyAction) {
	delete(m.bindings, action)
}

// GetAction 获取按键对应的动作
func (m *KeyBindingManager) GetAction(key tea.KeyType, alt, ctrl, shift bool) (KeyAction, bool) {
	for action, bindings := range m.bindings {
		for _, binding := range bindings {
			if m.matchBinding(binding, key, alt, ctrl, shift) {
				return action, true
			}
		}
	}
	return "", false
}

// matchBinding 检查按键是否匹配绑定
func (m *KeyBindingManager) matchBinding(binding *KeyBinding, key tea.KeyType, alt, ctrl, shift bool) bool {
	// 检查按键类型
	keyMatch := false
	for _, k := range binding.Keys {
		if k == key {
			keyMatch = true
			break
		}
	}
	if !keyMatch {
		return false
	}

	// 检查修饰键
	if binding.Alt && !alt {
		return false
	}
	if binding.Ctrl && !ctrl {
		return false
	}
	if binding.Shift && !shift {
		return false
	}

	return true
}

// SetVimMode 设置 Vim 模式
func (m *KeyBindingManager) SetVimMode(enabled bool) {
	m.vimMode = enabled

	if enabled {
		m.setupVimBindings()
	} else {
		m.setupDefaultBindings()
	}
}

// setupVimBindings 设置 Vim 模式绑定
func (m *KeyBindingManager) setupVimBindings() {
	// 清除现有绑定
	m.bindings = make(map[KeyAction][]*KeyBinding)

	// Vim 风格的导航 - 注释掉，因为需要特殊处理 rune
	// m.AddBinding(&KeyBinding{
	// 	Keys:   []tea.KeyType{tea.KeyRune}, // 将在运行时处理
	// 	Action: KeyUp,
	// })

	// Vim 模式命令
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyEsc},
		Action: KeyCancel,
	})

	// 使用其他键代替 : (冒号)
	// m.AddBinding(&KeyBinding{
	// 	Keys:   []tea.KeyType{tea.KeyColon},
	// 	Action: KeySearch,
	// })

	// 其他默认绑定
	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyCtrlC},
		Action: KeyQuit,
	})

	m.AddBinding(&KeyBinding{
		Keys:   []tea.KeyType{tea.KeyEnter},
		Action: KeySubmit,
	})
}

// GetBindingsForAction 获取动作的所有绑定
func (m *KeyBindingManager) GetBindingsForAction(action KeyAction) []*KeyBinding {
	return m.bindings[action]
}

// GetHelpText 获取帮助文本
func (m *KeyBindingManager) GetHelpText() string {
	// 实现帮助文本生成
	return "Key Bindings:\n" +
		"  Ctrl+C  - Quit\n" +
		"  Enter   - Submit\n" +
		"  Esc     - Cancel\n" +
		"  Up/Down - Navigate\n" +
		"  PgUp/PgDn - Page\n" +
		"  Ctrl+T  - New Tab\n" +
		"  Ctrl+W  - Close Tab\n" +
		"  Ctrl+S  - Save\n" +
		"  F1      - Help"
}

// IsVimMode 检查是否是 Vim 模式
func (m *KeyBindingManager) IsVimMode() bool {
	return m.vimMode
}
