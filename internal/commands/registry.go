package commands

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Command 定义命令接口
type Command interface {
	// Name 返回命令名称
	Name() string

	// Description 返回命令描述
	Description() string

	// Usage 返回使用说明
	Usage() string

	// Aliases 返回命令别名
	Aliases() []string

	// Execute 执行命令
	Execute(ctx *Context, args []string) (string, error)

	// Validate 验证参数
	Validate(args []string) error

	// Category 返回命令分类
	Category() string
}

// Context 命令执行上下文
type Context struct {
	// 可以在这里添加命令执行需要的各种上下文信息
	// 如：会话管理器、配置、日志器等
	Manager     interface{}
	Config      interface{}
	Logger      interface{}
}

// BaseCommand 基础命令实现
type BaseCommand struct {
	name        string
	description string
	usage       string
	aliases     []string
	category    string
	executeFn   func(ctx *Context, args []string) (string, error)
	validateFn  func(args []string) error
}

// NewBaseCommand 创建基础命令
func NewBaseCommand(name, description, usage string, aliases []string, category string,
	executeFn func(ctx *Context, args []string) (string, error),
	validateFn func(args []string) error) *BaseCommand {
	return &BaseCommand{
		name:        name,
		description: description,
		usage:       usage,
		aliases:     aliases,
		category:    category,
		executeFn:   executeFn,
		validateFn:  validateFn,
	}
}

// Name 返回命令名称
func (c *BaseCommand) Name() string {
	return c.name
}

// Description 返回命令描述
func (c *BaseCommand) Description() string {
	return c.description
}

// Usage 返回使用说明
func (c *BaseCommand) Usage() string {
	return c.usage
}

// Aliases 返回命令别名
func (c *BaseCommand) Aliases() []string {
	return c.aliases
}

// Category 返回命令分类
func (c *BaseCommand) Category() string {
	return c.category
}

// Execute 执行命令
func (c *BaseCommand) Execute(ctx *Context, args []string) (string, error) {
	if c.executeFn == nil {
		return "", fmt.Errorf("command not implemented")
	}
	return c.executeFn(ctx, args)
}

// Validate 验证参数
func (c *BaseCommand) Validate(args []string) error {
	if c.validateFn == nil {
		return nil
	}
	return c.validateFn(args)
}

// Registry 命令注册表
type Registry struct {
	commands    map[string]Command
	categories  map[string][]string
	fuzzyMatch  bool
	prefix      string
	mu          sync.RWMutex
}

// NewRegistry 创建命令注册表
func NewRegistry(options ...RegistryOption) *Registry {
	r := &Registry{
		commands:   make(map[string]Command),
		categories: make(map[string][]string),
		prefix:     "/",
		fuzzyMatch: true,
	}

	for _, opt := range options {
		opt(r)
	}

	return r
}

// RegistryOption 注册表配置选项
type RegistryOption func(*Registry)

// WithPrefix 设置命令前缀
func WithPrefix(prefix string) RegistryOption {
	return func(r *Registry) {
		r.prefix = prefix
	}
}

// WithFuzzyMatch 启用/禁用模糊匹配
func WithFuzzyMatch(enabled bool) RegistryOption {
	return func(r *Registry) {
		r.fuzzyMatch = enabled
	}
}

// Register 注册命令
func (r *Registry) Register(cmd Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	name := r.normalizeCommandName(cmd.Name())
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	r.commands[name] = cmd

	// 注册别名
	for _, alias := range cmd.Aliases() {
		aliasName := r.normalizeCommandName(alias)
		r.commands[aliasName] = cmd
	}

	// 添加到分类
	category := cmd.Category()
	if category != "" {
		r.categories[category] = append(r.categories[category], cmd.Name())
	}

	return nil
}

// Unregister 注销命令
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name = r.normalizeCommandName(name)

	cmd, ok := r.commands[name]
	if !ok {
		return fmt.Errorf("command not found: %s", name)
	}

	// 注销命令及其别名
	for k, v := range r.commands {
		if v == cmd {
			delete(r.commands, k)
		}
	}

	return nil
}

// Get 获取命令
func (r *Registry) Get(name string) (Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name = r.normalizeCommandName(name)
	cmd, ok := r.commands[name]
	return cmd, ok
}

// List 列出所有命令
func (r *Registry) List() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 去重
	seen := make(map[string]bool)
	var result []Command

	for _, cmd := range r.commands {
		name := cmd.Name()
		if !seen[name] {
			seen[name] = true
			result = append(result, cmd)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}

// ListByCategory 按分类列出命令
func (r *Registry) ListByCategory(category string) []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmds, ok := r.categories[category]
	if !ok {
		return nil
	}

	var result []Command
	for _, name := range cmds {
		if cmd, ok := r.commands[name]; ok {
			result = append(result, cmd)
		}
	}

	return result
}

// Categories 返回所有分类
func (r *Registry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cats := make([]string, 0, len(r.categories))
	for cat := range r.categories {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	return cats
}

// Search 搜索命令（支持模糊匹配）
func (r *Registry) Search(query string) []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = r.normalizeCommandName(query)
	if query == "" {
		return r.List()
	}

	var result []Command
	seen := make(map[string]bool)

	// 精确匹配
	if cmd, ok := r.commands[query]; ok {
		name := cmd.Name()
		if !seen[name] {
			seen[name] = true
			result = append(result, cmd)
		}
	}

	if !r.fuzzyMatch {
		return result
	}

	// 模糊匹配
	for name, cmd := range r.commands {
		if strings.Contains(name, query) {
			cmdName := cmd.Name()
			if !seen[cmdName] {
				seen[cmdName] = true
				result = append(result, cmd)
			}
		}
	}

	return result
}

// Execute 执行命令
func (r *Registry) Execute(ctx *Context, input string) (string, error) {
	// 解析输入
	cmdName, args := r.parseInput(input)

	// 获取命令
	cmd, ok := r.Get(cmdName)
	if !ok {
		// 尝试搜索
		matches := r.Search(cmdName)
		if len(matches) == 1 {
			cmd = matches[0]
		} else if len(matches) > 1 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name()
			}
			return "", fmt.Errorf("ambiguous command: %s, did you mean: %s?",
				cmdName, strings.Join(names, ", "))
		} else {
			return "", fmt.Errorf("unknown command: %s (use /help to see available commands)", cmdName)
		}
	}

	// 验证参数
	if err := cmd.Validate(args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w\nUsage: %s", err, cmd.Usage())
	}

	// 执行命令
	return cmd.Execute(ctx, args)
}

// IsCommand 检查输入是否是命令
func (r *Registry) IsCommand(input string) bool {
	input = strings.TrimSpace(input)
	if len(input) == 0 {
		return false
	}

	return strings.HasPrefix(input, r.prefix)
}

// normalizeCommandName 规范化命令名称
func (r *Registry) normalizeCommandName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, r.prefix)
	name = strings.ToLower(name)
	return name
}

// parseInput 解析输入
func (r *Registry) parseInput(input string) (string, []string) {
	input = strings.TrimSpace(input)

	// 移除前缀
	input = strings.TrimPrefix(input, r.prefix)

	// 分割命令和参数
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	cmdName := parts[0]
	args := parts[1:]

	return cmdName, args
}

// GetHelp 获取帮助信息
func (r *Registry) GetHelp(category string) string {
	var cmds []Command
	if category != "" {
		cmds = r.ListByCategory(category)
		if len(cmds) == 0 {
			return fmt.Sprintf("No commands found in category: %s", category)
		}
	} else {
		cmds = r.List()
	}

	var sb strings.Builder
	sb.WriteString("Available commands:\n\n")

	// 按分类分组
	catGroups := make(map[string][]Command)
	for _, cmd := range cmds {
		cat := cmd.Category()
		if cat == "" {
			cat = "General"
		}
		catGroups[cat] = append(catGroups[cat], cmd)
	}

	// 按分类排序
	cats := make([]string, 0, len(catGroups))
	for cat := range catGroups {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	// 输出每个分类
	for _, cat := range cats {
		sb.WriteString(fmt.Sprintf("  [%s]\n", cat))
		for _, cmd := range catGroups[cat] {
			aliases := ""
			if len(cmd.Aliases()) > 0 {
				aliases = fmt.Sprintf(" (%s)", strings.Join(cmd.Aliases(), ", "))
			}
			sb.WriteString(fmt.Sprintf("    /%-20s%s%s\n",
				cmd.Name(),
				cmd.Description(),
				aliases))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetCommandHelp 获取特定命令的帮助
func (r *Registry) GetCommandHelp(name string) string {
	cmd, ok := r.Get(name)
	if !ok {
		return fmt.Sprintf("Command not found: %s", name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Command: %s\n\n", cmd.Name()))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", cmd.Description()))
	sb.WriteString(fmt.Sprintf("Usage: %s\n", cmd.Usage()))

	if len(cmd.Aliases()) > 0 {
		sb.WriteString(fmt.Sprintf("\nAliases: %s\n", strings.Join(cmd.Aliases(), ", ")))
	}

	return sb.String()
}
