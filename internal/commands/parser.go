package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ParseResult 解析结果
type ParseResult struct {
	Positional []string         // 位置参数
	Flags      map[string]interface{} // 标志参数
	RawArgs    []string         // 原始参数
}

// Parser 参数解析器
type Parser struct {
	flags map[string]*FlagSpec
	args  []*ArgSpec
}

// FlagSpec 标志规范
type FlagSpec struct {
	Name         string      // 标志名称（长形式）
	Short        string      // 标志简称
	Description  string      // 描述
	DefaultValue interface{} // 默认值
	Required     bool        // 是否必需
	Type         string      // 类型: string, int, bool, float
	Aliases      []string    // 别名
}

// ArgSpec 位置参数规范
type ArgSpec struct {
	Name        string  // 参数名称
	Description string   // 描述
	Required    bool     // 是否必需
	Variadic    bool     // 是否可变参数
	Default     string   // 默认值
	Parser      func(string) (interface{}, error) // 自定义解析器
}

// NewParser 创建解析器
func NewParser() *Parser {
	return &Parser{
		flags: make(map[string]*FlagSpec),
		args:  make([]*ArgSpec, 0),
	}
}

// AddFlag 添加标志
func (p *Parser) AddFlag(spec *FlagSpec) *Parser {
	p.flags[spec.Name] = spec

	// 添加简称映射
	if spec.Short != "" {
		p.flags[spec.Short] = spec
	}

	// 添加别名映射
	for _, alias := range spec.Aliases {
		p.flags[alias] = spec
	}

	return p
}

// AddArg 添加位置参数
func (p *Parser) AddArg(spec *ArgSpec) *Parser {
	p.args = append(p.args, spec)
	return p
}

// Parse 解析参数
func (p *Parser) Parse(args []string) (*ParseResult, error) {
	result := &ParseResult{
		Positional: make([]string, 0),
		Flags:      make(map[string]interface{}),
		RawArgs:    args,
	}

	// 设置默认值
	for name, spec := range p.flags {
		if spec.DefaultValue != nil {
			result.Flags[name] = spec.DefaultValue
		}
	}

	i := 0

	// 解析位置参数
	for _, argSpec := range p.args {
		if i >= len(args) {
			if argSpec.Required {
				return nil, fmt.Errorf("missing required argument: %s", argSpec.Name)
			}
			if argSpec.Default != "" {
				result.Positional = append(result.Positional, argSpec.Default)
			}
			continue
		}

		// 检查是否是标志（以 - 开头）
		if strings.HasPrefix(args[i], "-") {
			if argSpec.Required {
				return nil, fmt.Errorf("missing required argument: %s", argSpec.Name)
			}
			continue
		}

		if argSpec.Variadic {
			// 可变参数，收集所有剩余参数（直到遇到标志）
			for i < len(args) && !strings.HasPrefix(args[i], "-") {
				result.Positional = append(result.Positional, args[i])
				i++
			}
		} else {
			result.Positional = append(result.Positional, args[i])
			i++
		}
	}

	// 解析标志
	for i < len(args) {
		arg := args[i]

		// 检查是否是标志
		if !strings.HasPrefix(arg, "-") {
			result.Positional = append(result.Positional, args[i:]...)
			break
		}

		// 解析标志名称
		flagName := strings.TrimLeft(arg, "-")

		// 查找标志规范
		spec, ok := p.flags[flagName]
		if !ok {
			return nil, fmt.Errorf("unknown flag: %s", arg)
		}

		// 布尔标志
		if spec.Type == "bool" {
			result.Flags[spec.Name] = true
			i++
			continue
		}

		// 需要值的标志
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag %s requires a value", arg)
		}

		value := args[i+1]
		parsedValue, err := p.parseValue(value, spec)
		if err != nil {
			return nil, fmt.Errorf("invalid value for flag %s: %w", arg, err)
		}

		result.Flags[spec.Name] = parsedValue
		i += 2
	}

	// 验证必需标志
	for name, spec := range p.flags {
		if spec.Required {
			if _, ok := result.Flags[name]; !ok {
				// 检查是否有任何别名或简称被设置
				found := false
				for flagKey := range result.Flags {
					if fs, ok := p.flags[flagKey]; ok && fs == spec {
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("required flag not set: %s", name)
				}
			}
		}
	}

	return result, nil
}

// parseValue 解析值
func (p *Parser) parseValue(value string, spec *FlagSpec) (interface{}, error) {
	switch spec.Type {
	case "string":
		return value, nil
	case "int":
		return strconv.Atoi(value)
	case "float":
		return strconv.ParseFloat(value, 64)
	case "bool":
		return strconv.ParseBool(value)
	default:
		return value, nil
	}
}

// GetString 获取字符串值
func (r *ParseResult) GetString(name string, defaultValue string) string {
	if val, ok := r.Flags[name]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetInt 获取整数值
func (r *ParseResult) GetInt(name string, defaultValue int) int {
	if val, ok := r.Flags[name]; ok {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return defaultValue
}

// GetFloat 获取浮点数值
func (r *ParseResult) GetFloat(name string, defaultValue float64) float64 {
	if val, ok := r.Flags[name]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return defaultValue
}

// GetBool 获取布尔值
func (r *ParseResult) GetBool(name string, defaultValue bool) bool {
	if val, ok := r.Flags[name]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// GetPositional 获取位置参数
func (r *ParseResult) GetPositional(index int, defaultValue string) string {
	if index < 0 || index >= len(r.Positional) {
		return defaultValue
	}
	return r.Positional[index]
}

// GetPositionalSlice 获取位置参数切片
func (r *ParseResult) GetPositionalSlice(start, end int) []string {
	if start < 0 {
		start = 0
	}
	if end > len(r.Positional) {
		end = len(r.Positional)
	}
	if start >= end {
		return nil
	}
	return r.Positional[start:end]
}

// Validate 验证解析结果
func (r *ParseResult) Validate(specs []*ArgSpec) error {
	// 检查必需的位置参数
	for i, spec := range specs {
		if spec.Required && i >= len(r.Positional) {
			return errors.New("missing required argument: " + spec.Name)
		}
	}
	return nil
}

// Help 生成帮助信息
func (p *Parser) Help(commandName string) string {
	var sb strings.Builder
	sb.WriteString("Usage: ")
	sb.WriteString(commandName)

	// 位置参数
	for _, arg := range p.args {
		sb.WriteString(" ")
		if arg.Variadic {
			sb.WriteString("[")
			sb.WriteString(arg.Name)
			sb.WriteString("...]")
		} else if arg.Required {
			sb.WriteString("<")
			sb.WriteString(arg.Name)
			sb.WriteString(">")
		} else {
			sb.WriteString("[")
			sb.WriteString(arg.Name)
			sb.WriteString("]")
		}
	}

	// 标志
	if len(p.flags) > 0 {
		sb.WriteString(" [flags]")
	}

	sb.WriteString("\n\n")

	// 标志说明
	if len(p.flags) > 0 {
		sb.WriteString("Flags:\n")
		for name, spec := range p.flags {
			sb.WriteString("  ")
			if spec.Short != "" {
				sb.WriteString("-")
				sb.WriteString(spec.Short)
				sb.WriteString(", ")
			}
			sb.WriteString("--")
			sb.WriteString(name)

			if spec.Type != "bool" {
				sb.WriteString(" <value>")
			}

			sb.WriteString("    ")
			sb.WriteString(spec.Description)

			if spec.DefaultValue != nil {
				sb.WriteString(" (default: ")
				sb.WriteString(fmt.Sprintf("%v", spec.DefaultValue))
				sb.WriteString(")")
			}

			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ParseSimple 简单参数解析（不使用规范）
func ParseSimple(args []string) (*ParseResult, error) {
	result := &ParseResult{
		Positional: make([]string, 0),
		Flags:      make(map[string]interface{}),
		RawArgs:    args,
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "-") {
			// 标志
			flagName := strings.TrimLeft(arg, "-")

			// 检查是否是 --flag=value 形式
			if idx := strings.Index(flagName, "="); idx != -1 {
				name := flagName[:idx]
				value := flagName[idx+1:]
				result.Flags[name] = value
				i++
				continue
			}

			// 检查是否是布尔标志
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				result.Flags[flagName] = true
				i++
				continue
			}

			// 带值的标志
			value := args[i+1]
			// 尝试解析为数字
			if intVal, err := strconv.Atoi(value); err == nil {
				result.Flags[flagName] = intVal
			} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
				result.Flags[flagName] = floatVal
			} else if boolVal, err := strconv.ParseBool(value); err == nil {
				result.Flags[flagName] = boolVal
			} else {
				result.Flags[flagName] = value
			}
			i += 2
		} else {
			// 位置参数
			result.Positional = append(result.Positional, arg)
			i++
		}
	}

	return result, nil
}
