package chat

import (
	"time"
)

// Role 消息角色类型
type Role string

const (
	RoleSystem    Role = "system"    // 系统提示
	RoleUser      Role = "user"      // 用户消息
	RoleAssistant Role = "assistant" // 助手回复
	RoleTool      Role = "tool"      // 工具调用结果
)

// Message 对话消息
type Message struct {
	ID        string                 // 消息唯一ID
	Role      Role                   // 消息角色
	Content  string                 // 消息内容
	Timestamp time.Time              // 时间戳
	TokenCount int                    // Token 数量
	Metadata  map[string]interface{} // 元数据
}

// NewMessage 创建新消息
func NewMessage(role Role, content string) *Message {
	return &Message{
		ID:        generateID(),
		Role:      role,
		Content:  content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// WithID 设置消息 ID
func (m *Message) WithID(id string) *Message {
	m.ID = id
	return m
}

// WithTokenCount 设置 Token 数量
func (m *Message) WithTokenCount(count int) *Message {
	m.TokenCount = count
	return m
}

// WithMetadata 添加元数据
func (m *Message) WithMetadata(key string, value interface{}) *Message {
	m.Metadata[key] = value
	return m
}

// GetMetadata 获取元数据
func (m *Message) GetMetadata(key string) (interface{}, bool) {
	v, ok := m.Metadata[key]
	return v, ok
}

// IsUser 是否为用户消息
func (m *Message) IsUser() bool {
	return m.Role == RoleUser
}

// IsAssistant 是否为助手消息
func (m *Message) IsAssistant() bool {
	return m.Role == RoleAssistant
}

// IsSystem 是否为系统消息
func (m *Message) IsSystem() bool {
	return m.Role == RoleSystem
}

// Clone 克隆消息
func (m *Message) Clone() *Message {
	cloned := &Message{
		ID:        m.ID,
		Role:      m.Role,
		Content:  m.Content,
		Timestamp: m.Timestamp,
		TokenCount: m.TokenCount,
	}
	if m.Metadata != nil {
		cloned.Metadata = make(map[string]interface{})
		for k, v := range m.Metadata {
			cloned.Metadata[k] = v
		}
	}
	return cloned
}

// generateID 生成唯一 ID
func generateID() string {
	return "msg_" + time.Now().Format("20060102150405.000000")
}
