package builtin

import (
	"fmt"
	"linkyun-edge-proxy/internal/commands"
	"strings"
)

// HelpCommand 帮助命令
type HelpCommand struct {
	registry *commands.Registry
}

// NewHelpCommand 创建帮助命令
func NewHelpCommand(registry *commands.Registry) *HelpCommand {
	return &HelpCommand{registry: registry}
}

// Name 返回命令名称
func (c *HelpCommand) Name() string {
	return "help"
}

// Description 返回命令描述
func (c *HelpCommand) Description() string {
	return "Display help information"
}

// Usage 返回使用说明
func (c *HelpCommand) Usage() string {
	return "/help [category|command]"
}

// Aliases 返回命令别名
func (c *HelpCommand) Aliases() []string {
	return []string{"h", "?"}
}

// Category 返回命令分类
func (c *HelpCommand) Category() string {
	return "General"
}

// Execute 执行命令
func (c *HelpCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) == 0 {
		// 显示所有命令分类
		categories := c.registry.Categories()
		var sb strings.Builder
		sb.WriteString("Available command categories:\n\n")
		for _, cat := range categories {
			sb.WriteString(fmt.Sprintf("  • %s\n", cat))
		}
		sb.WriteString("\nUse /help <category> to see commands in a specific category")
		sb.WriteString("\nUse /help <command> to see detailed command help")
		sb.WriteString("\n\nRun /help General to see all commands")
		return sb.String(), nil
	}

	// 检查是否是特定命令
	cmd, ok := c.registry.Get(args[0])
	if ok {
		return c.registry.GetCommandHelp(cmd.Name()), nil
	}

	// 检查是否是分类
	help := c.registry.GetHelp(args[0])
	return help, nil
}

// Validate 验证参数
func (c *HelpCommand) Validate(args []string) error {
	// 参数可选
	return nil
}
