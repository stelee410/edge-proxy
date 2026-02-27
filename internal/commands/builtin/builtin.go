package builtin

import (
	"linkyun-edge-proxy/internal/chat"
	"linkyun-edge-proxy/internal/commands"
)

// RegisterBuiltinCommands 注册所有内置命令
func RegisterBuiltinCommands(registry *commands.Registry, manager *chat.Manager, chatClient interface{}, config interface{}) {
	// General 命令
	registry.Register(NewHelpCommand(registry))

	// Session 命令
	registry.Register(NewHistoryCommand(manager))
	registry.Register(NewClearCommand(manager))
	registry.Register(NewResetCommand(manager))
	registry.Register(NewSaveSessionCommand(manager))
	registry.Register(NewLoadSessionCommand(manager))
	registry.Register(NewListSessionsCommand(manager))
	registry.Register(NewDeleteSessionCommand(manager))
	registry.Register(NewRenameSessionCommand(manager))
	registry.Register(NewNewSessionCommand(manager))
	registry.Register(NewSwitchSessionCommand(manager))
	registry.Register(NewExportCommand(manager))

	// Config 命令
	registry.Register(NewModelCommand(chatClient))
	registry.Register(NewTemperatureCommand(chatClient))
	registry.Register(NewSystemCommand(chatClient))
	registry.Register(NewSettingsCommand(config))

	// AI 命令
	registry.Register(NewAskCommand(chatClient))
	registry.Register(NewSummarizeCommand(chatClient))
	registry.Register(NewExplainCommand(chatClient))
	registry.Register(NewRefactorCommand(chatClient))
	registry.Register(NewReviewCommand(chatClient))
	registry.Register(NewFixCommand(chatClient))
	registry.Register(NewTestCommand(chatClient))
	registry.Register(NewOptimizeCommand(chatClient))
	registry.Register(NewDocumentCommand(chatClient))
	registry.Register(NewTranslateCommand(chatClient))
	registry.Register(NewAnalyzeCommand(chatClient))
	registry.Register(NewDiffCommand(chatClient))

	// Info 命令
	registry.Register(NewTrendingCommand(chatClient))
}
