package rules

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontmatter MDC 文件的 YAML frontmatter 结构
type frontmatter struct {
	Description string `yaml:"description"`
	AlwaysApply bool   `yaml:"alwaysApply"`
	Globs       string `yaml:"globs"`    // 逗号分隔的 glob 模式
	Priority    int    `yaml:"priority"` // 优先级，默认 0
	Name        string `yaml:"name"`     // 可选，规则名称
}

// ParseMDC 解析 .mdc 文件内容，提取 frontmatter 元数据和正文
// filePath 用于在未指定 name 时从文件名推导规则名称
func ParseMDC(content string, filePath string) (*Rule, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	var meta frontmatter
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}
	}

	// 规则名称：优先使用 frontmatter 中的 name，否则从文件名推导
	name := meta.Name
	if name == "" && filePath != "" {
		base := filepath.Base(filePath)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// 解析 globs（逗号分隔）
	var globs []string
	if meta.Globs != "" {
		for _, g := range strings.Split(meta.Globs, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				globs = append(globs, g)
			}
		}
	}

	return &Rule{
		Name:        name,
		Description: meta.Description,
		Content:     strings.TrimSpace(body),
		AlwaysApply: meta.AlwaysApply,
		Globs:       globs,
		Priority:    meta.Priority,
		FilePath:    filePath,
	}, nil
}

// splitFrontmatter 分离 frontmatter 和正文
// frontmatter 由 --- 分隔符包围
func splitFrontmatter(content string) (frontmatter string, body string, err error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	// 找到第二个 ---
	rest := content[3:] // 跳过第一个 ---
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return "", content, nil
	}

	fm := strings.TrimSpace(rest[:idx])
	body = strings.TrimSpace(rest[idx+4:]) // 跳过 \n---

	return fm, body, nil
}

// ParsePriority 从字符串解析优先级
func ParsePriority(s string) int {
	if s == "" {
		return 0
	}
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return p
}
