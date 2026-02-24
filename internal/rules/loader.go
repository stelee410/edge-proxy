package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Loader 规则文件加载器
type Loader struct {
	directories []string
}

// NewLoader 创建规则加载器
func NewLoader(directories []string) *Loader {
	return &Loader{
		directories: directories,
	}
}

// Load 扫描所有配置目录，加载并解析 .mdc 文件，返回 RuleSet
func (l *Loader) Load() (*RuleSet, error) {
	ruleSet := NewRuleSet()

	for _, dir := range l.directories {
		rules, err := l.loadDirectory(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to load rules from %q: %w", dir, err)
		}
		for _, r := range rules {
			ruleSet.Add(r)
		}
	}

	// 按优先级排序（越小越优先）
	sort.Slice(ruleSet.Rules, func(i, j int) bool {
		if ruleSet.Rules[i].Priority != ruleSet.Rules[j].Priority {
			return ruleSet.Rules[i].Priority < ruleSet.Rules[j].Priority
		}
		return ruleSet.Rules[i].Name < ruleSet.Rules[j].Name
	})

	return ruleSet, nil
}

// loadDirectory 扫描单个目录，递归查找并加载所有 .mdc 文件
func (l *Loader) loadDirectory(dir string) ([]*Rule, error) {
	var rules []*Rule

	// 检查目录是否存在
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 目录不存在，静默跳过
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 只处理 .mdc 文件
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".mdc") {
			return nil
		}

		rule, err := l.loadFile(path)
		if err != nil {
			return fmt.Errorf("failed to load %q: %w", path, err)
		}
		if rule != nil {
			rules = append(rules, rule)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return rules, nil
}

// loadFile 加载并解析单个 .mdc 文件
func (l *Loader) loadFile(path string) (*Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	rule, err := ParseMDC(string(data), path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return rule, nil
}
