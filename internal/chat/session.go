package chat

import (
	"sync"
	"time"
)

// SessionConfig 会话配置
type SessionConfig struct {
	ID          string
	Name        string
	Description string
	Model       string
	Temperature float64
	MaxTokens   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Session 会话
type Session struct {
	config  *SessionConfig
	context *Context
	mu      sync.RWMutex
	active  bool
}

// NewSession 创建新会话
func NewSession(config *SessionConfig) *Session {
	if config == nil {
		config = &SessionConfig{
			ID:        "session_" + time.Now().Format("20060102150405"),
			Name:      "New Session",
			Model:     "zhipu-glm5",
			Temperature: 0.7,
			MaxTokens:  4000,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	if config.ID == "" {
		config.ID = "session_" + time.Now().Format("20060102150405")
	}

	session := &Session{
		config: config,
		active: true,
	}

	// 创建上下文
	session.context = NewContext(&ContextConfig{
		SystemPrompt:   "",
		MaxMessages:    50,
		MaxTokens:      config.MaxTokens,
		EnableSummary:  true,
		SummaryTrigger: 30,
	})

	return session
}

// GetID 获取会话 ID
func (s *Session) GetID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.ID
}

// GetName 获取会话名称
func (s *Session) GetName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Name
}

// SetName 设置会话名称
func (s *Session) SetName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Name = name
	s.config.UpdatedAt = time.Now()
}

// GetModel 获取模型
func (s *Session) GetModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Model
}

// SetModel 设置模型
func (s *Session) SetModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Model = model
	s.config.UpdatedAt = time.Now()
}

// GetTemperature 获取温度参数
func (s *Session) GetTemperature() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Temperature
}

// SetTemperature 设置温度参数
func (s *Session) SetTemperature(temp float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Temperature = temp
	s.config.UpdatedAt = time.Now()
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.context.AddMessage(msg)
	s.config.UpdatedAt = time.Now()
}

// GetMessages 获取所有消息
func (s *Session) GetMessages() []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.context.GetMessages()
}

// GetContext 获取对话上下文
func (s *Session) GetContext() *Context {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.context
}

// IsActive 检查会话是否活跃
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.active
}

// SetActive 设置会话活跃状态
func (s *Session) SetActive(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.active = active
	s.config.UpdatedAt = time.Now()
}

// Clear 清空会话（保留系统提示）
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.context.Clear()
	s.config.UpdatedAt = time.Now()
}

// GetConfig 获取会话配置副本
func (s *Session) GetConfig() SessionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return *s.config
}

// UpdateConfig 更新会话配置
func (s *Session) UpdateConfig(config SessionConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config.ID != "" {
		s.config.ID = config.ID
	}
	if config.Name != "" {
		s.config.Name = config.Name
	}
	if config.Description != "" {
		s.config.Description = config.Description
	}
	if config.Model != "" {
		s.config.Model = config.Model
	}
	if config.Temperature >= 0 {
		s.config.Temperature = config.Temperature
	}
	if config.MaxTokens > 0 {
		s.config.MaxTokens = config.MaxTokens
	}
	s.config.UpdatedAt = time.Now()
}

// Clone 克隆会话
func (s *Session) Clone() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	newConfig := *s.config
	newConfig.ID = "session_" + time.Now().Format("20060102150405")
	newConfig.CreatedAt = time.Now()
	newConfig.UpdatedAt = time.Now()

	newSession := &Session{
		config: &newConfig,
		active: true,
	}

	// 克隆消息
	messages := s.context.GetMessages()
	newSession.context = NewContext(nil)
	for _, msg := range messages {
		newSession.context.AddMessage(msg.Clone())
	}

	return newSession
}
