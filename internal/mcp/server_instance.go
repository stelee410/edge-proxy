package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// ServerStatus 服务器状态
type ServerStatus string

const (
	StatusDisconnected ServerStatus = "disconnected"
	StatusConnecting   ServerStatus = "connecting"
	StatusReady        ServerStatus = "ready"
	StatusError        ServerStatus = "error"
)

const maxRestartAttempts = 3

// ServerConfig MCP 服务器配置
type ServerConfig struct {
	Name      string            `yaml:"name"`
	Transport string            `yaml:"transport"` // "stdio" | "sse"
	Command   string            `yaml:"command"`   // stdio: 可执行文件
	Args      []string          `yaml:"args"`      // stdio: 命令行参数
	Env       map[string]string `yaml:"env"`       // stdio: 环境变量
	WorkDir   string            `yaml:"work_dir"`  // stdio: 工作目录
	URL       string            `yaml:"url"`       // sse: 服务器 URL
	Headers   map[string]string `yaml:"headers"`   // sse: 自定义请求头
	Resources ResourceConfig    `yaml:"resources"` // 资源配置
}

// ServerInstance 单个 MCP 服务器实例
type ServerInstance struct {
	config    ServerConfig
	client    *Client
	tools     []MCPTool
	resources []MCPResource
	status    ServerStatus
	lastError error
	mu        sync.RWMutex
	restarts  int
}

// NewServerInstance 创建服务器实例
func NewServerInstance(cfg ServerConfig) *ServerInstance {
	return &ServerInstance{
		config: cfg,
		status: StatusDisconnected,
	}
}

// Start 启动并初始化 MCP 服务器连接
func (s *ServerInstance) Start(ctx context.Context) error {
	s.mu.Lock()
	s.status = StatusConnecting
	s.mu.Unlock()

	transport, err := s.createTransport(ctx)
	if err != nil {
		s.mu.Lock()
		s.status = StatusError
		s.lastError = err
		s.mu.Unlock()
		return fmt.Errorf("failed to create transport for %q: %w", s.config.Name, err)
	}

	client := NewClient(transport)

	if err := client.Initialize(ctx); err != nil {
		client.Close()
		s.mu.Lock()
		s.status = StatusError
		s.lastError = err
		s.mu.Unlock()
		return fmt.Errorf("failed to initialize %q: %w", s.config.Name, err)
	}

	// 获取工具列表
	tools, err := client.ListTools(ctx)
	if err != nil {
		logger.Warn("MCP server %q: failed to list tools: %v", s.config.Name, err)
		tools = nil
	}

	// 获取资源列表
	resources, err := client.ListResources(ctx)
	if err != nil {
		logger.Debug("MCP server %q: failed to list resources: %v (may not support resources)", s.config.Name, err)
		resources = nil
	}

	s.mu.Lock()
	s.client = client
	s.tools = tools
	s.resources = resources
	s.status = StatusReady
	s.lastError = nil
	s.mu.Unlock()

	logger.Info("MCP server %q ready: %d tools, %d resources",
		s.config.Name, len(tools), len(resources))

	return nil
}

// createTransport 根据配置创建传输层
func (s *ServerInstance) createTransport(ctx context.Context) (Transport, error) {
	switch s.config.Transport {
	case "stdio":
		return NewStdioTransport(ctx, StdioConfig{
			Command: s.config.Command,
			Args:    s.config.Args,
			Env:     s.config.Env,
			WorkDir: s.config.WorkDir,
		})
	case "sse":
		return NewSSETransport(SSEConfig{
			URL:     s.config.URL,
			Headers: s.config.Headers,
		})
	default:
		return nil, fmt.Errorf("unknown transport type %q", s.config.Transport)
	}
}

// Restart 重启服务器连接（带重试限制）
func (s *ServerInstance) Restart(ctx context.Context) error {
	s.mu.Lock()
	if s.restarts >= maxRestartAttempts {
		s.mu.Unlock()
		return fmt.Errorf("max restart attempts (%d) reached for %q", maxRestartAttempts, s.config.Name)
	}
	s.restarts++
	attempt := s.restarts
	s.mu.Unlock()

	logger.Info("MCP server %q: restarting (attempt %d/%d)...", s.config.Name, attempt, maxRestartAttempts)

	// 关闭旧连接
	s.mu.RLock()
	if s.client != nil {
		s.client.Close()
	}
	s.mu.RUnlock()

	// 等待一小段时间再重连
	time.Sleep(time.Duration(attempt) * time.Second)

	return s.Start(ctx)
}

// CallTool 调用工具
func (s *ServerInstance) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPToolResult, error) {
	s.mu.RLock()
	client := s.client
	status := s.status
	s.mu.RUnlock()

	if status != StatusReady || client == nil {
		return nil, fmt.Errorf("MCP server %q is not ready (status: %s)", s.config.Name, status)
	}

	return client.CallTool(ctx, name, args)
}

// Tools 返回工具列表
func (s *ServerInstance) Tools() []MCPTool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tools
}

// Resources 返回资源列表
func (s *ServerInstance) Resources() []MCPResource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resources
}

// Status 返回当前状态
func (s *ServerInstance) Status() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Name 返回服务器名称
func (s *ServerInstance) Name() string {
	return s.config.Name
}

// Close 关闭服务器连接
func (s *ServerInstance) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = StatusDisconnected
	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		return err
	}
	return nil
}
