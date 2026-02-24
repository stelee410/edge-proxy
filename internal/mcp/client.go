package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// Client MCP 客户端
type Client struct {
	transport  Transport
	idGen      IDGenerator
	serverInfo *InitializeResult

	// 请求-响应映射
	pending  map[int64]chan *Response
	mu       sync.Mutex
	closed   bool
	stopOnce sync.Once
	done     chan struct{}
}

// NewClient 创建 MCP 客户端
func NewClient(transport Transport) *Client {
	c := &Client{
		transport: transport,
		pending:   make(map[int64]chan *Response),
		done:      make(chan struct{}),
	}

	go c.readLoop()

	return c
}

// readLoop 持续读取传输层消息
func (c *Client) readLoop() {
	defer close(c.done)

	for {
		data, err := c.transport.Receive()
		if err != nil {
			if !c.closed {
				logger.Debug("MCP client read error: %v", err)
			}
			return
		}

		// 尝试解析为响应
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			logger.Debug("MCP client: received non-JSON-RPC message: %s", string(data))
			continue
		}

		// 如果有 ID，匹配到 pending 请求
		if resp.ID > 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()

			if ok {
				ch <- &resp
			}
		}
	}
}

// call 发送请求并等待响应
func (c *Client) call(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := c.idGen.Next()

	req := NewRequest(id, method, params)
	data, err := EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// 注册 pending 通道
	ch := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	// 发送请求
	if err := c.transport.Send(data); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 等待响应
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout for method %q", method)
	}
}

// notify 发送通知（不等待响应）
func (c *Client) notify(method string, params interface{}) error {
	n := NewNotification(method, params)
	data, err := EncodeNotification(n)
	if err != nil {
		return fmt.Errorf("failed to encode notification: %w", err)
	}
	return c.transport.Send(data)
}

// Initialize 初始化 MCP 连接
func (c *Client) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    Capabilities{},
		ClientInfo: Implementation{
			Name:    ClientName,
			Version: ClientVersion,
		},
	}

	resp, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	result, err := DecodeResult[InitializeResult](resp)
	if err != nil {
		return fmt.Errorf("failed to decode initialize result: %w", err)
	}

	c.serverInfo = result
	logger.Info("MCP server: %s %s (protocol: %s)",
		result.ServerInfo.Name, result.ServerInfo.Version, result.ProtocolVersion)

	// 发送 initialized 通知
	if err := c.notify("notifications/initialized", nil); err != nil {
		logger.Warn("Failed to send initialized notification: %v", err)
	}

	return nil
}

// ListTools 获取工具列表
func (c *Client) ListTools(ctx context.Context) ([]MCPTool, error) {
	resp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list failed: %w", err)
	}

	result, err := DecodeResult[ToolsListResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tools list: %w", err)
	}

	return result.Tools, nil
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPToolResult, error) {
	params := ToolCallParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tools/call failed: %w", err)
	}

	result, err := DecodeResult[MCPToolResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tool call result: %w", err)
	}

	return result, nil
}

// ListResources 获取资源列表
func (c *Client) ListResources(ctx context.Context) ([]MCPResource, error) {
	resp, err := c.call(ctx, "resources/list", nil)
	if err != nil {
		return nil, fmt.Errorf("resources/list failed: %w", err)
	}

	result, err := DecodeResult[ResourcesListResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resources list: %w", err)
	}

	return result.Resources, nil
}

// ReadResource 读取资源内容
func (c *Client) ReadResource(ctx context.Context, uri string) (*MCPResourceContent, error) {
	params := ResourceReadParams{URI: uri}

	resp, err := c.call(ctx, "resources/read", params)
	if err != nil {
		return nil, fmt.Errorf("resources/read failed: %w", err)
	}

	result, err := DecodeResult[ResourceReadResult](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resource content: %w", err)
	}

	if len(result.Contents) == 0 {
		return nil, fmt.Errorf("no content returned for resource %q", uri)
	}

	return &result.Contents[0], nil
}

// ServerInfo 返回服务器信息
func (c *Client) ServerInfo() *InitializeResult {
	return c.serverInfo
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.stopOnce.Do(func() {
		c.closed = true
		c.transport.Close()
	})

	// 等待 readLoop 退出
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
	}

	return nil
}
