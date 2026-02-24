package rules

// Rule 表示一条 Agent 规则，从 .mdc 文件解析而来
type Rule struct {
	Name        string   `json:"name"`         // 规则名称（来自 frontmatter 或文件名）
	Description string   `json:"description"`  // 规则描述
	Content     string   `json:"content"`      // 规则正文（Markdown 内容）
	AlwaysApply bool     `json:"always_apply"` // 是否始终应用
	Globs       []string `json:"globs"`        // 匹配 glob 模式（用于条件应用）
	Priority    int      `json:"priority"`     // 优先级（越小越优先，默认 0）
	FilePath    string   `json:"file_path"`    // 源文件路径
}

// RuleSet 一组规则的集合
type RuleSet struct {
	Rules []*Rule
}

// NewRuleSet 创建空的 RuleSet
func NewRuleSet() *RuleSet {
	return &RuleSet{
		Rules: make([]*Rule, 0),
	}
}

// Add 添加规则
func (rs *RuleSet) Add(r *Rule) {
	rs.Rules = append(rs.Rules, r)
}

// GetAlwaysApply 返回所有 alwaysApply=true 的规则
func (rs *RuleSet) GetAlwaysApply() []*Rule {
	var result []*Rule
	for _, r := range rs.Rules {
		if r.AlwaysApply {
			result = append(result, r)
		}
	}
	return result
}

// GetByName 根据名称查找规则
func (rs *RuleSet) GetByName(name string) *Rule {
	for _, r := range rs.Rules {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// Count 返回规则总数
func (rs *RuleSet) Count() int {
	return len(rs.Rules)
}
