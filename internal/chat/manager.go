package chat

import (
	"fmt"
	"sync"
	"time"
)

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DefaultModel    string
	DefaultTemp     float64
	MaxSessions     int
	EnablePersistence bool
	StoragePath     string
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		DefaultModel:     "zhipu-glm5",
		DefaultTemp:      0.7,
		MaxSessions:     10,
		EnablePersistence: true,
		StoragePath:     "./data/sessions/",
	}
}

// Manager 会话管理器
type Manager struct {
	config   *ManagerConfig
	sessions map[string]*Session
	activeID string
	mu       sync.RWMutex
}

// NewManager 创建会话管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	return &Manager{
		config:   config,
		sessions: make(map[string]*Session),
	}
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(name string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否超过最大会话数
	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum number of sessions (%d) reached", m.config.MaxSessions)
	}

	config := &SessionConfig{
		ID:          "session_" + time.Now().Format("20060102150405"),
		Name:        name,
		Model:       m.config.DefaultModel,
		Temperature: m.config.DefaultTemp,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	session := NewSession(config)
	m.sessions[session.GetID()] = session

	// 如果没有活跃会话，设置为活跃
	if m.activeID == "" {
		m.activeID = session.GetID()
	}

	return session, nil
}

// GetSession 获取指定会话
func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	return session, ok
}

// GetActiveSession 获取当前活跃会话
func (m *Manager) GetActiveSession() (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.activeID == "" {
		return nil, false
	}

	session, ok := m.sessions[m.activeID]
	return session, ok
}

// SetActiveSession 设置活跃会话
func (m *Manager) SetActiveSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// 将之前的会话设为非活跃
	if m.activeID != "" && m.sessions[m.activeID] != nil {
		m.sessions[m.activeID].SetActive(false)
	}

	m.activeID = id
	m.sessions[id].SetActive(true)

	return nil
}

// ListSessions 列出所有会话
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// 如果删除的是活跃会话，需要选择另一个
	if m.activeID == id {
		// 找到第一个非活跃的会话
		for sessionID, session := range m.sessions {
			if sessionID != id {
				m.activeID = sessionID
				session.SetActive(true)
				break
			}
		}
		// 如果没有其他会话，清空活跃ID
		if m.activeID == id {
			m.activeID = ""
		}
	}

	delete(m.sessions, id)
	return nil
}

// ClearSession 清空会话内容
func (m *Manager) ClearSession(id string) error {
	m.mu.Lock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.Clear()
	return nil
}

// RenameSession 重命名会话
func (m *Manager) RenameSession(id, name string) error {
	m.mu.Lock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.SetName(name)
	return nil
}

// GetSessionCount 获取会话数量
func (m *Manager) GetSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions)
}

// GetActiveSessionID 获取活跃会话 ID
func (m *Manager) GetActiveSessionID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeID
}

// CleanupInactive 清理不活跃的会话
func (m *Manager) CleanupInactive() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var inactiveIDs []string
	for id, session := range m.sessions {
		context := session.GetContext()
		if context.IsIdle() && !session.IsActive() {
			inactiveIDs = append(inactiveIDs, id)
		}
	}

	for _, id := range inactiveIDs {
		delete(m.sessions, id)
	}

	return len(inactiveIDs)
}

// UpdateDefaultConfig 更新默认配置
func (m *Manager) UpdateDefaultConfig(config ManagerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.DefaultModel != "" {
		m.config.DefaultModel = config.DefaultModel
	}
	if config.DefaultTemp >= 0 {
		m.config.DefaultTemp = config.DefaultTemp
	}
	if config.MaxSessions > 0 {
		m.config.MaxSessions = config.MaxSessions
	}
	if config.StoragePath != "" {
		m.config.StoragePath = config.StoragePath
	}
	m.config.EnablePersistence = config.EnablePersistence
}
