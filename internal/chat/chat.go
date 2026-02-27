package chat

import (
	"context"
	"fmt"
)

// LLMProvider LLM 提供商接口
type LLMProvider interface {
	// Chat 发送聊天请求
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// StreamChat 流式聊天
	StreamChat(ctx context.Context, req *ChatRequest, handler *StreamHandler) error
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string     // 模型名称
	Messages    []*Message // 消息列表
	Temperature float64    // 温度参数
	MaxTokens   int        // 最大 Token 数
	Stream      bool       // 是否流式输出
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content     string    // 响应内容
	Message     *Message  // 完整消息
	TokenCount  int       // Token 数量
	Model       string    // 使用的模型
	FinishReason string   // 结束原因
}

// ChatClient 聊天客户端
type ChatClient struct {
	provider LLMProvider
	manager  *Manager
}

// NewChatClient 创建聊天客户端
func NewChatClient(provider LLMProvider, manager *Manager) *ChatClient {
	return &ChatClient{
		provider: provider,
		manager:  manager,
	}
}

// Send 发送消息
func (c *ChatClient) Send(ctx context.Context, sessionID string, content string) (*ChatResponse, error) {
	// 获取会话
	session, ok := c.manager.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 创建用户消息
	userMsg := NewMessage(RoleUser, content)
	session.AddMessage(userMsg)

	// 获取上下文消息
	chatCtx := session.GetContext()
	messages := chatCtx.GetMessages()

	// 构建请求
	req := &ChatRequest{
		Model:       session.GetModel(),
		Messages:    messages,
		Temperature: session.GetTemperature(),
		MaxTokens:   session.GetConfig().MaxTokens,
		Stream:      false,
	}

	// 发送请求
	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request: %w", err)
	}

	// 创建助手消息
	assistantMsg := NewMessage(RoleAssistant, resp.Content)
	assistantMsg.WithTokenCount(resp.TokenCount)
	session.AddMessage(assistantMsg)

	// 更新响应消息引用
	resp.Message = assistantMsg

	return resp, nil
}

// StreamSend 流式发送消息
func (c *ChatClient) StreamSend(ctx context.Context, sessionID string, content string) (*StreamHandler, error) {
	// 获取会话
	session, ok := c.manager.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 创建用户消息
	userMsg := NewMessage(RoleUser, content)
	session.AddMessage(userMsg)

	// 获取上下文消息
	chatCtx := session.GetContext()
	messages := chatCtx.GetMessages()

	// 构建请求
	req := &ChatRequest{
		Model:       session.GetModel(),
		Messages:    messages,
		Temperature: session.GetTemperature(),
		MaxTokens:   session.GetConfig().MaxTokens,
		Stream:      true,
	}

	// 创建流处理器
	handler := NewStreamHandler()

	// 异步发送请求
	go func() {
		defer func() {
			if r := recover(); r != nil {
				handler.Error(fmt.Errorf("panic: %v", r))
			}
		}()

		if err := c.provider.StreamChat(ctx, req, handler); err != nil {
			handler.Error(err)
			return
		}
	}()

	return handler, nil
}

// Retry 重试最后一条消息
func (c *ChatClient) Retry(ctx context.Context, sessionID string) (*ChatResponse, error) {
	// 获取会话
	session, ok := c.manager.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 获取最后一条用户消息
	messages := session.GetMessages()
	var lastUserMsg *Message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].IsUser() {
			lastUserMsg = messages[i]
			break
		}
	}

	if lastUserMsg == nil {
		return nil, fmt.Errorf("no user message found")
	}

	// 重新发送
	return c.Send(ctx, sessionID, lastUserMsg.Content)
}

// Regenerate 重新生成最后一条助手回复
func (c *ChatClient) Regenerate(ctx context.Context, sessionID string) (*ChatResponse, error) {
	// 获取会话
	session, ok := c.manager.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 获取消息列表
	messages := session.GetMessages()

	// 移除最后一条助手消息
	if len(messages) == 0 || !messages[len(messages)-1].IsAssistant() {
		return nil, fmt.Errorf("no assistant message to regenerate")
	}

	// 移除最后一条消息（助手回复）
	newMessages := messages[:len(messages)-1]

	// 更新会话消息
	session.context.mu.Lock()
	session.context.messages = newMessages
	session.context.mu.Unlock()

	// 获取最后一条用户消息重新发送
	var lastUserMsg *Message
	for i := len(newMessages) - 1; i >= 0; i-- {
		if newMessages[i].IsUser() {
			lastUserMsg = newMessages[i]
			break
		}
	}

	if lastUserMsg == nil {
		return nil, fmt.Errorf("no user message found")
	}

	return c.Send(ctx, sessionID, lastUserMsg.Content)
}
