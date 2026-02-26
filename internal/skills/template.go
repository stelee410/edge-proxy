package skills

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// RenderTemplate 渲染 Go text/template 模板
// data 为模板变量 map，支持嵌套
func RenderTemplate(tmplStr string, data map[string]interface{}) (string, error) {
	if tmplStr == "" {
		return "", nil
	}

	// 注入内置变量
	if data == nil {
		data = make(map[string]interface{})
	}
	data["_now"] = time.Now().Format(time.RFC3339)
	data["_date"] = time.Now().Format("2006-01-02")
	data["_time"] = time.Now().Format("15:04:05")

	funcMap := template.FuncMap{
		"join":         strings.Join,
		"upper":        strings.ToUpper,
		"lower":        strings.ToLower,
		"trim":         strings.TrimSpace,
		"contains":     strings.Contains,
		"replace":      strings.ReplaceAll,
		"default":      templateDefault,
		"addIfNotZero": addIfNotZero,
		"len":          templateLen,
	}

	tmpl, err := template.New("skill").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// templateDefault 模板函数：返回 value，如果为空则返回 defaultVal
func templateDefault(defaultVal, value interface{}) interface{} {
	if value == nil {
		return defaultVal
	}
	if s, ok := value.(string); ok && s == "" {
		return defaultVal
	}
	return value
}

// addIfNotZero 如果 value 不为空，则返回 "+" + value，否则返回空字符串
func addIfNotZero(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok && s != "" {
		return "+" + s
	}
	return ""
}

// templateLen 返回值的长度，支持 string, map, slice, array
func templateLen(value interface{}) int {
	if value == nil {
		return 0
	}
	switch v := value.(type) {
	case string:
		return len(v)
	case []interface{}:
		return len(v)
	case map[string]interface{}:
		return len(v)
	default:
		// 尝试通过反射获取长度
		return 0
	}
}

// RenderURL 渲染 URL 模板（简化版，用 {{.key}} 替换）
func RenderURL(urlTemplate string, data map[string]interface{}) (string, error) {
	return RenderTemplate(urlTemplate, data)
}
