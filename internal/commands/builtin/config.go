package builtin

import (
	"fmt"
	"linkyun-edge-proxy/internal/commands"
	"strings"
)

// ModelCommand 模型切换命令
type ModelCommand struct {
	manager interface{}
}

// NewModelCommand 创建模型命令
func NewModelCommand(manager interface{}) *ModelCommand {
	return &ModelCommand{manager: manager}
}

func (c *ModelCommand) Name() string              { return "model" }
func (c *ModelCommand) Description() string       { return "Set or view the current model" }
func (c *ModelCommand) Usage() string            { return "/model [model_name]" }
func (c *ModelCommand) Aliases() []string        { return []string{} }
func (c *ModelCommand) Category() string        { return "Config" }

func (c *ModelCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	// TODO: 实现模型切换逻辑
	if len(args) == 0 {
		return "Current model: zhipu-glm5", nil
	}
	return fmt.Sprintf("Model set to: %s", args[0]), nil
}

func (c *ModelCommand) Validate(args []string) error {
	return nil
}

// TemperatureCommand 温度参数命令
type TemperatureCommand struct {
	manager interface{}
}

// NewTemperatureCommand 创建温度命令
func NewTemperatureCommand(manager interface{}) *TemperatureCommand {
	return &TemperatureCommand{manager: manager}
}

func (c *TemperatureCommand) Name() string              { return "temperature" }
func (c *TemperatureCommand) Description() string       { return "Set the temperature parameter (0.0 - 2.0)" }
func (c *TemperatureCommand) Usage() string            { return "/temperature <value>" }
func (c *TemperatureCommand) Aliases() []string        { return []string{"temp"} }
func (c *TemperatureCommand) Category() string        { return "Config" }

func (c *TemperatureCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "Current temperature: 0.7", nil
	}

	// TODO: 验证温度值范围
	return fmt.Sprintf("Temperature set to: %s", args[0]), nil
}

func (c *TemperatureCommand) Validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("temperature value is required")
	}
	return nil
}

// SystemCommand 系统提示命令
type SystemCommand struct {
	manager interface{}
}

// NewSystemCommand 创建系统提示命令
func NewSystemCommand(manager interface{}) *SystemCommand {
	return &SystemCommand{manager: manager}
}

func (c *SystemCommand) Name() string              { return "system" }
func (c *SystemCommand) Description() string       { return "Set the system prompt" }
func (c *SystemCommand) Usage() string            { return "/system <prompt>" }
func (c *SystemCommand) Aliases() []string        { return []string{"sys"} }
func (c *SystemCommand) Category() string        { return "Config" }

func (c *SystemCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) == 0 {
		// TODO: 获取当前系统提示
		return "Current system prompt: You are a helpful assistant.", nil
	}

	prompt := args[0]
	// TODO: 设置系统提示
	return fmt.Sprintf("System prompt set to: %s", prompt), nil
}

func (c *SystemCommand) Validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("system prompt is required")
	}
	return nil
}

// SettingsCommand 设置命令
type SettingsCommand struct {
	config interface{}
}

// NewSettingsCommand 创建设置命令
func NewSettingsCommand(config interface{}) *SettingsCommand {
	return &SettingsCommand{config: config}
}

func (c *SettingsCommand) Name() string              { return "settings" }
func (c *SettingsCommand) Description() string       { return "Open or view settings" }
func (c *SettingsCommand) Usage() string            { return "/settings" }
func (c *SettingsCommand) Aliases() []string        { return []string{"config", "cfg"} }
func (c *SettingsCommand) Category() string        { return "Config" }

func (c *SettingsCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	// TODO: 显示配置详情
	var sb strings.Builder
	sb.WriteString("Current Settings:\n\n")
	sb.WriteString("  Model: zhipu-glm5\n")
	sb.WriteString("  Temperature: 0.7\n")
	sb.WriteString("  Max Tokens: 4000\n")
	sb.WriteString("  Theme: dark\n")
	sb.WriteString("  Language: en\n")
	return sb.String(), nil
}

func (c *SettingsCommand) Validate(args []string) error {
	return nil
}
