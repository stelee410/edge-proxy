package chat

import (
	"sync"
	"time"
)

// ContextConfig 上下文配置
type ContextConfig struct {
	MaxMessages    int           // 最大保留消息数
	MaxTokens      int           // 最大 Token 数
	SystemPrompt   string        // 系统提示
	EnableSummary  bool          // 是否启用摘要压缩
	SummaryTrigger int           // 触发摘要的消息数
	IdleTimeout    time.Duration // 空闲超时
}

// DefaultContextConfig 默认上下文配置
func DefaultContextConfig() *ContextConfig {
	return &ContextConfig{
		MaxMessages:    50,
		MaxTokens:      4000,
		SystemPrompt:   "You are a helpful assistant.",
		EnableSummary:  true,
		SummaryTrigger: 30,
		IdleTimeout:    30 * time.Minute,
	}
}

// Context 对话上下文
type Context struct {
	messages      []*Message
	config        *ContextConfig
	mu            sync.RWMutex
	lastActivity  time.Time
	tokenCount    int
	summary       string // 摘要内容
}

// NewContext 创建新的对话上下文
func NewContext(config *ContextConfig) *Context {
	if config == nil {
		config = DefaultContextConfig()
	}

	ctx := &Context{
		messages:     make([]*Message, 0, config.MaxMessages),
		config:       config,
		lastActivity: time.Now(),
	}

	// 添加系统提示
	if config.SystemPrompt != "" {
		ctx.AddMessage(NewMessage(RoleSystem, config.SystemPrompt))
	}

	return ctx
}

// AddMessage 添加消息到上下文
func (c *Context) AddMessage(msg *Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, msg)
	c.lastActivity = time.Now()
	c.tokenCount += msg.TokenCount

	// 检查是否需要压缩
	if c.config.EnableSummary && len(c.messages) > c.config.SummaryTrigger {
		c.compress()
	}

	// 检查是否超出最大消息数
	if len(c.messages) > c.config.MaxMessages {
		// 保留系统消息和最近的消息
		systemMsgs := c.filterMessagesByRole(RoleSystem)
		recentMsgs := c.messages[len(c.messages)-c.config.MaxMessages:]

		c.messages = append(systemMsgs, recentMsgs...)
	}
}

// GetMessages 获取所有消息
func (c *Context) GetMessages() []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// GetLastMessage 获取最后一条消息
func (c *Context) GetLastMessage() *Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.messages) == 0 {
		return nil
	}
	return c.messages[len(c.messages)-1]
}

// GetLastNMessages 获取最后 N 条消息
func (c *Context) GetLastNMessages(n int) []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.messages) == 0 {
		return nil
	}

	start := len(c.messages) - n
	if start < 0 {
		start = 0
	}

	result := make([]*Message, len(c.messages)-start)
	copy(result, c.messages[start:])
	return result
}

// GetMessagesByRole 获取指定角色的消息
func (c *Context) GetMessagesByRole(role Role) []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.filterMessagesByRole(role)
}

// filterMessagesByRole 过滤指定角色的消息（需要持有锁）
func (c *Context) filterMessagesByRole(role Role) []*Message {
	var result []*Message
	for _, msg := range c.messages {
		if msg.Role == role {
			result = append(result, msg)
		}
	}
	return result
}

// Clear 清空上下文（保留系统消息）
func (c *Context) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	systemMsgs := c.filterMessagesByRole(RoleSystem)
	c.messages = systemMsgs
	c.tokenCount = 0
	c.lastActivity = time.Now()
}

// GetTokenCount 获取当前 Token 总数
func (c *Context) GetTokenCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.tokenCount
}

// GetMessageCount 获取消息数量
func (c *Context) GetMessageCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.messages)
}

// GetLastActivity 获取最后活动时间
func (c *Context) GetLastActivity() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastActivity
}

// IsIdle 检查是否空闲
func (c *Context) IsIdle() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.config.IdleTimeout <= 0 {
		return false
	}
	return time.Since(c.lastActivity) > c.config.IdleTimeout
}

// UpdateConfig 更新配置
func (c *Context) UpdateConfig(config *ContextConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config = config
}

// compress 压缩上下文（需要持有锁）
func (c *Context) compress() {
	// 保留系统消息
	systemMsgs := c.filterMessagesByRole(RoleSystem)
	if len(c.messages) <= 10 {
		c.messages = systemMsgs
		return
	}

	// 保留最后几条消息用于后续对话
	lastMsgs := c.messages[len(c.messages)-10:]

	// 计算摘要（这里简化处理，实际应调用 LLM 生成摘要）
	c.summary = "Previous conversation context: " + c.generateSimpleSummary()

	// 重建消息列表
	c.messages = append(systemMsgs, lastMsgs...)
}

// generateSimpleSummary 生成简单摘要
func (c *Context) generateSimpleSummary() string {
	userMsgs := c.filterMessagesByRole(RoleUser)
	assistantMsgs := c.filterMessagesByRole(RoleAssistant)

	return "User sent " + string(rune(len(userMsgs))) + " messages, assistant responded " + string(rune(len(assistantMsgs))) + " times."
}

// SetSystemPrompt 设置系统提示
func (c *Context) SetSystemPrompt(prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 移除旧的系统消息
	var systemMsgs []*Message
	var otherMsgs []*Message
	for _, msg := range c.messages {
		if msg.IsSystem() {
			systemMsgs = append(systemMsgs, msg)
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}

	// 添加新的系统消息
	c.messages = append(systemMsgs[:0]) // 清空
	c.messages = append(c.messages, NewMessage(RoleSystem, prompt))
	c.messages = append(c.messages, otherMsgs...)
}
