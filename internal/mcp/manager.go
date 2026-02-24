package mcp

import (
	"context"
	"fmt"
	"sync"

	"linkyun-edge-proxy/internal/logger"
)

// Manager MCP 服务器管理器，管理多个 MCP 服务器连接
type Manager struct {
	servers map[string]*ServerInstance
	mu      sync.RWMutex
}

// NewManager 创建 MCP 管理器
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*ServerInstance),
	}
}

// Start 启动所有配置的 MCP 服务器
func (m *Manager) Start(ctx context.Context, configs []ServerConfig) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(configs))

	for _, cfg := range configs {
		instance := NewServerInstance(cfg)
		m.mu.Lock()
		m.servers[cfg.Name] = instance
		m.mu.Unlock()

		wg.Add(1)
		go func(inst *ServerInstance) {
			defer wg.Done()
			if err := inst.Start(ctx); err != nil {
				logger.Warn("MCP server %q failed to start: %v", inst.Name(), err)
				errCh <- err
			}
		}(instance)
	}

	wg.Wait()
	close(errCh)

	// 收集错误但不阻止整体启动
	var failCount int
	for err := range errCh {
		_ = err
		failCount++
	}

	readyCount := len(configs) - failCount
	logger.Info("MCP Manager: %d/%d servers ready", readyCount, len(configs))

	if readyCount == 0 && len(configs) > 0 {
		return fmt.Errorf("no MCP servers started successfully")
	}

	return nil
}

// GetServer 获取指定名称的服务器实例
func (m *Manager) GetServer(name string) *ServerInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers[name]
}

// GetAllTools 获取所有服务器的工具列表
// 工具名会添加服务器名前缀以避免冲突，格式: serverName__toolName
func (m *Manager) GetAllTools() []MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []MCPTool
	for serverName, instance := range m.servers {
		if instance.Status() != StatusReady {
			continue
		}
		for _, tool := range instance.Tools() {
			prefixedTool := MCPTool{
				Name:        serverName + "__" + tool.Name,
				Description: tool.Description,
				InputSchema: tool.InputSchema,
			}
			allTools = append(allTools, prefixedTool)
		}
	}
	return allTools
}

// CallTool 调用指定服务器的工具
// toolName 格式: serverName__toolName
func (m *Manager) CallTool(ctx context.Context, qualifiedName string, args map[string]interface{}) (*MCPToolResult, error) {
	serverName, toolName := splitToolName(qualifiedName)
	if serverName == "" {
		return nil, fmt.Errorf("invalid tool name %q: must be in format 'serverName__toolName'", qualifiedName)
	}

	m.mu.RLock()
	instance, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server %q not found", serverName)
	}

	return instance.CallTool(ctx, toolName, args)
}

// ServerNames 返回所有服务器名称
func (m *Manager) ServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// ServerCount 返回服务器数量
func (m *Manager) ServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.servers)
}

// Shutdown 关闭所有服务器连接
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, instance := range m.servers {
		if err := instance.Close(); err != nil {
			logger.Warn("MCP server %q close error: %v", name, err)
			lastErr = err
		}
	}

	logger.Info("MCP Manager: all servers shut down")
	return lastErr
}

// splitToolName 分割 qualified tool name 为 serverName 和 toolName
func splitToolName(qualifiedName string) (string, string) {
	for i := 0; i < len(qualifiedName)-1; i++ {
		if qualifiedName[i] == '_' && qualifiedName[i+1] == '_' {
			return qualifiedName[:i], qualifiedName[i+2:]
		}
	}
	return "", qualifiedName
}
