package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"linkyun-edge-proxy/internal/chat"
	"linkyun-edge-proxy/internal/commands"
	"linkyun-edge-proxy/internal/config"
	"linkyun-edge-proxy/internal/llm"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/mcp"
	"linkyun-edge-proxy/internal/proxy"
	"linkyun-edge-proxy/internal/rules"
	"linkyun-edge-proxy/internal/sandbox"
	"linkyun-edge-proxy/internal/skills"
	"linkyun-edge-proxy/internal/tui"
	builtinCmds "linkyun-edge-proxy/internal/commands/builtin"
)

func main() {
	configPath := flag.String("config", "edge-proxy-config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 创建通信通道
	logChan := make(chan logger.LogEntry, 1000)
	statsChan := make(chan proxy.ProxyStats, 100)

	// 初始化日志缓冲器
	logger.InitBuffer(10000, logChan)
	logger.SetLevel(cfg.LogLevel)

	// 禁用标准输出，所有日志只通过 channel 发送到 TUI
	logger.DisableStdout()

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
	p.SetStatsChannel(statsChan)

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

	// 初始化 Bash 沙箱（若配置启用，则暴露 run_shell 工具）
	if cfg.Sandbox.Enabled {
		sbCfg := sandbox.Config{
			WorkDir:        cfg.Sandbox.WorkDir,
			TimeoutSeconds: cfg.Sandbox.TimeoutSeconds,
			BashCommand:    cfg.Sandbox.BashCommand,
			ExtraBlacklist: cfg.Sandbox.ExtraBlacklist,
		}
		sb, err := sandbox.New(sbCfg)
		if err != nil {
			logger.Warn("Failed to init sandbox: %v", err)
		} else {
			p.SetSandbox(sb)
			logger.Info("Sandbox enabled: run_shell tool available")
		}
	}

	// 初始化 MCP 管理器（如果配置启用）
	var mcpManager *mcp.Manager
	if cfg.MCP.Enabled && len(cfg.MCP.Servers) > 0 {
		mcpManager = mcp.NewManager()
		logger.Info("MCP enabled with %d server(s)", len(cfg.MCP.Servers))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	// 启动 Proxy 在后台 goroutine
	proxyDone := make(chan struct{})
	var proxyErr error
	go func() {
		proxyErr = p.Run(ctx)
		close(proxyDone) // 用 close 而不是发送值，这样多处都能收到
	}()

	// 初始化聊天管理器
	chatManager := chat.NewManager(nil)
	// 创建默认会话
	if _, err := chatManager.CreateSession("Default"); err != nil {
		logger.Warn("Failed to create default session: %v", err)
	}

	// 初始化命令注册表
	cmdRegistry := commands.NewRegistry(
		commands.WithPrefix("/"),
		commands.WithFuzzyMatch(true),
	)

	// 注册内置命令
	builtinCmds.RegisterBuiltinCommands(cmdRegistry, chatManager, nil, cfg)

	// 启动 Bubble Tea TUI（主线程）- 使用增强型模型，传入 p 以支持聊天界面本地测试 LLM + Skill
	tuiModel := tui.NewEnhancedModel(logChan, statsChan, cfg, chatManager, cmdRegistry, p)
	// 不使用 WithMouseAllMotion，以便在日志模式下可用鼠标选择文本并复制
	program := tea.NewProgram(
		tuiModel,
		tea.WithAltScreen(),
	)

	// 信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-quit:
			logger.Info("Received shutdown signal")
			program.Quit()
			cancel()
		case <-proxyDone:
			if proxyErr != nil {
				logger.Error("Proxy exited with error: %v", proxyErr)
			}
			program.Quit()
		}
	}()

	// 运行 TUI（阻塞）
	if finalModel, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		cancel()
		<-proxyDone
		if mcpManager != nil {
			mcpManager.Shutdown()
		}
		os.Exit(1)
	} else {
		// 检查是否是用户主动退出
		if enhancedModel, ok := finalModel.(*tui.EnhancedModel); ok && enhancedModel.IsQuit() {
			cancel()
		}
	}

	// 等待 Proxy 优雅退出
	<-proxyDone

	// MCP 清理
	if mcpManager != nil {
		mcpManager.Shutdown()
	}
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
