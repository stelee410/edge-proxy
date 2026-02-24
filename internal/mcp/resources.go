package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// ResourceConfig MCP 资源配置
type ResourceConfig struct {
	AutoInject bool `yaml:"auto_inject"` // 自动注入所有资源
	MaxSize    int  `yaml:"max_size"`    // 单个资源最大字节数，默认 10KB
	CacheTTL   int  `yaml:"cache_ttl"`   // 缓存时间（秒），默认 300
}

// DefaultResourceConfig 返回默认资源配置
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		AutoInject: false,
		MaxSize:    10240, // 10KB
		CacheTTL:   300,   // 5 分钟
	}
}

// cachedResource 缓存的资源内容
type cachedResource struct {
	content   string
	fetchedAt time.Time
}

// ResourceManager MCP 资源管理器
type ResourceManager struct {
	manager  *Manager
	config   map[string]ResourceConfig // server name -> resource config
	cache    map[string]*cachedResource
	mu       sync.RWMutex
}

// NewResourceManager 创建资源管理器
func NewResourceManager(mgr *Manager) *ResourceManager {
	return &ResourceManager{
		manager: mgr,
		config:  make(map[string]ResourceConfig),
		cache:   make(map[string]*cachedResource),
	}
}

// SetServerConfig 设置服务器的资源配置
func (rm *ResourceManager) SetServerConfig(serverName string, cfg ResourceConfig) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config[serverName] = cfg
}

// BuildResourceContext 构建所有 auto_inject 资源的上下文文本
func (rm *ResourceManager) BuildResourceContext(ctx context.Context) string {
	rm.mu.RLock()
	configs := make(map[string]ResourceConfig)
	for k, v := range rm.config {
		configs[k] = v
	}
	rm.mu.RUnlock()

	var parts []string

	for serverName, cfg := range configs {
		if !cfg.AutoInject {
			continue
		}

		instance := rm.manager.GetServer(serverName)
		if instance == nil || instance.Status() != StatusReady {
			continue
		}

		resources := instance.Resources()
		if len(resources) == 0 {
			continue
		}

		for _, res := range resources {
			content, err := rm.getResourceContent(ctx, serverName, res.URI, cfg)
			if err != nil {
				logger.Debug("MCP resource %q from %q: read failed: %v", res.URI, serverName, err)
				continue
			}
			if content == "" {
				continue
			}

			label := res.Name
			if label == "" {
				label = res.URI
			}

			parts = append(parts, fmt.Sprintf("<mcp-resource server=%q name=%q uri=%q>\n%s\n</mcp-resource>",
				serverName, label, res.URI, content))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "## MCP Resources\n\n" + strings.Join(parts, "\n\n")
}

// getResourceContent 获取资源内容（带缓存）
func (rm *ResourceManager) getResourceContent(ctx context.Context, serverName, uri string, cfg ResourceConfig) (string, error) {
	cacheKey := serverName + "::" + uri

	// 检查缓存
	rm.mu.RLock()
	cached, ok := rm.cache[cacheKey]
	rm.mu.RUnlock()

	ttl := time.Duration(cfg.CacheTTL) * time.Second
	if ok && time.Since(cached.fetchedAt) < ttl {
		return cached.content, nil
	}

	// 从 MCP 服务器读取
	instance := rm.manager.GetServer(serverName)
	if instance == nil {
		return "", fmt.Errorf("server %q not found", serverName)
	}

	rm.mu.RLock()
	client := instance.client
	rm.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("server %q not connected", serverName)
	}

	resourceContent, err := client.ReadResource(ctx, uri)
	if err != nil {
		return "", err
	}

	content := resourceContent.Text
	if content == "" && resourceContent.Blob != "" {
		content = "[binary content]"
	}

	// 截断超大资源
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = 10240
	}
	if len(content) > maxSize {
		logger.Warn("MCP resource %q from %q: truncated from %d to %d bytes",
			uri, serverName, len(content), maxSize)
		content = content[:maxSize] + "\n... [truncated]"
	}

	// 更新缓存
	rm.mu.Lock()
	rm.cache[cacheKey] = &cachedResource{
		content:   content,
		fetchedAt: time.Now(),
	}
	rm.mu.Unlock()

	return content, nil
}

// ClearCache 清空资源缓存
func (rm *ResourceManager) ClearCache() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.cache = make(map[string]*cachedResource)
}

// InjectIntoSystemPrompt 将资源内容注入到 system prompt
func (rm *ResourceManager) InjectIntoSystemPrompt(ctx context.Context, systemPrompt string) string {
	resourceContext := rm.BuildResourceContext(ctx)
	if resourceContext == "" {
		return systemPrompt
	}

	if systemPrompt == "" {
		return resourceContext
	}

	return systemPrompt + "\n\n" + resourceContext
}
