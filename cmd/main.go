package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"linkyun-edge-proxy/internal/config"
	"linkyun-edge-proxy/internal/llm"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/mcp"
	"linkyun-edge-proxy/internal/proxy"
	"linkyun-edge-proxy/internal/rules"
	"linkyun-edge-proxy/internal/skills"
)

func main() {
	configPath := flag.String("config", "edge-proxy-config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.SetLevel(cfg.LogLevel)

	logger.Info("Linkyun Edge Proxy starting...")
	logger.Info("Agent UUID: %s", cfg.AgentUUID)
	logger.Info("Server: %s", cfg.ServerURL)
	logger.Info("Edge Token: %s", logger.MaskToken(cfg.EdgeToken))

	// 创建 Provider Registry 并注册内置工厂
	registry := llm.NewRegistry()
	registry.RegisterBuiltinFactories()

	// 从配置初始化 Providers
	providerConfigs, defaultName := cfg.LLM.GetProviderConfigs()
	if err := registry.InitProviders(providerConfigs, defaultName); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize LLM providers: %v\n", err)
		os.Exit(1)
	}

	logger.Info("LLM Providers initialized: [%s], default: %s",
		strings.Join(registry.ProviderNames(), ", "), registry.DefaultName())

	p := proxy.New(cfg, registry)

	// 初始化 Rules 引擎（如果配置启用）
	if cfg.Rules.Enabled && len(cfg.Rules.Directories) > 0 {
		rulesEngine := rules.NewEngine(cfg.Rules.Directories)
		if err := rulesEngine.LoadRules(); err != nil {
			logger.Warn("Failed to load rules: %v", err)
		} else {
			p.SetRulesEngine(rulesEngine)
			logger.Info("Rules loaded: %d rules from %v", rulesEngine.RuleCount(), cfg.Rules.Directories)
		}
	}

	// 初始化 Skills 管道（如果配置启用）
	// 全局配置传递给 code handler（如 TTS handler 需要 LLM 的 api_key 做 fallback）
	if cfg.Skills.Enabled && cfg.Skills.Directory != "" {
		globalCfg := buildGlobalSkillConfig(cfg)
		skillRegistry := skills.NewRegistry()
		count, err := skills.LoadAndRegisterSkills(cfg.Skills.Directory, skillRegistry, globalCfg)
		if err != nil {
			logger.Warn("Failed to load skills: %v", err)
		} else {
			pipeline := skills.NewPipeline(skillRegistry)
			p.SetSkillPipeline(pipeline)
			logger.Info("Skills loaded: %d skills from %s", count, cfg.Skills.Directory)
		}
	}

	// 确保 ToolExecutor 存在（当 Skills 未启用时，用于 MCP 等工具）
	p.EnsureToolExecutor()

	// 初始化 MCP 管理器（如果配置启用）
	var mcpManager *mcp.Manager
	if cfg.MCP.Enabled && len(cfg.MCP.Servers) > 0 {
		mcpManager = mcp.NewManager()
		logger.Info("MCP enabled with %d server(s)", len(cfg.MCP.Servers))
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 启动 Rules 热加载（如果启用）
	if cfg.Rules.Enabled && len(cfg.Rules.Directories) > 0 && p.GetRulesEngine() != nil {
		if err := p.GetRulesEngine().StartWatching(ctx); err != nil {
			logger.Warn("Failed to start rules watcher: %v", err)
		} else {
			logger.Info("Rules hot-reload watcher started")
		}
	}
	// 启动 MCP 服务器
	if mcpManager != nil {
		if err := mcpManager.Start(ctx, cfg.MCP.Servers); err != nil {
			logger.Warn("MCP Manager start failed: %v (continuing without MCP)", err)
		} else {
			p.SetMCPManager(mcpManager)

			// 初始化 MCP 资源管理器
			resourceMgr := mcp.NewResourceManager(mcpManager)
			for _, serverCfg := range cfg.MCP.Servers {
				resourceMgr.SetServerConfig(serverCfg.Name, serverCfg.Resources)
			}
			p.SetResourceManager(resourceMgr)

			logger.Info("MCP Manager: %d server(s) started, %d tools available",
				mcpManager.ServerCount(), len(mcpManager.GetAllTools()))
		}
	}
	defer func() {
		if mcpManager != nil {
			mcpManager.Shutdown()
		}
	}()
	defer cancel()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("Received shutdown signal")
		cancel()
	}()

	if err := p.Run(ctx); err != nil {
		logger.Error("Proxy exited with error: %v", err)
		os.Exit(1)
	}
	logger.Info("Edge Proxy stopped.")
}

// buildGlobalSkillConfig 构建全局配置 map，供 code skill handler 做 fallback
func buildGlobalSkillConfig(cfg *config.Config) map[string]interface{} {
	globalCfg := make(map[string]interface{})

	// 从旧格式 LLM 配置中取 api_key 和 base_url
	if cfg.LLM.APIKey != "" {
		globalCfg["llm_api_key"] = cfg.LLM.APIKey
	}
	if cfg.LLM.BaseURL != "" {
		globalCfg["llm_base_url"] = cfg.LLM.BaseURL
	}

	// 从新格式 LLM providers 中取第一个作为 fallback
	if len(cfg.LLM.Providers) > 0 {
		first := cfg.LLM.Providers[0]
		if _, ok := globalCfg["llm_api_key"]; !ok && first.APIKey != "" {
			globalCfg["llm_api_key"] = first.APIKey
		}
		if _, ok := globalCfg["llm_base_url"]; !ok && first.BaseURL != "" {
			globalCfg["llm_base_url"] = first.BaseURL
		}
	}

	return globalCfg
}
