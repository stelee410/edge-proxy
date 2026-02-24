package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRulesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	rule1 := `---
description: Always applied rule
alwaysApply: true
priority: 1
---

# Always Rule
You must always follow this rule.`

	rule2 := `---
description: Conditional rule
alwaysApply: false
globs: "*.go"
priority: 2
---

# Go Files Rule
This applies only to Go files.`

	rule3 := `---
description: Another always rule
alwaysApply: true
priority: 0
---

# Priority Zero Rule
This has highest priority.`

	os.WriteFile(filepath.Join(dir, "always-rule.mdc"), []byte(rule1), 0644)
	os.WriteFile(filepath.Join(dir, "conditional-rule.mdc"), []byte(rule2), 0644)
	os.WriteFile(filepath.Join(dir, "priority-rule.mdc"), []byte(rule3), 0644)

	return dir
}

func TestEngineLoadRules(t *testing.T) {
	dir := setupTestRulesDir(t)
	engine := NewEngine([]string{dir})

	if err := engine.LoadRules(); err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	if engine.RuleCount() != 3 {
		t.Errorf("expected 3 rules, got %d", engine.RuleCount())
	}
}

func TestEngineGetApplicableRules(t *testing.T) {
	dir := setupTestRulesDir(t)
	engine := NewEngine([]string{dir})

	if err := engine.LoadRules(); err != nil {
		t.Fatal(err)
	}

	applicable := engine.GetApplicableRules()
	if len(applicable) != 2 {
		t.Errorf("expected 2 always-apply rules, got %d", len(applicable))
	}
}

func TestEngineBuildContext(t *testing.T) {
	dir := setupTestRulesDir(t)
	engine := NewEngine([]string{dir})

	if err := engine.LoadRules(); err != nil {
		t.Fatal(err)
	}

	ctx := engine.BuildContext()
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}

	if !strings.Contains(ctx, "--- Rules ---") {
		t.Error("expected context to contain '--- Rules ---' separator")
	}
	if !strings.Contains(ctx, "[Rule 1:") {
		t.Error("expected context to contain '[Rule 1:'")
	}
	if !strings.Contains(ctx, "[Rule 2:") {
		t.Error("expected context to contain '[Rule 2:'")
	}
}

func TestEngineBuildContextEmpty(t *testing.T) {
	engine := NewEngine([]string{"/nonexistent"})
	engine.LoadRules()

	ctx := engine.BuildContext()
	if ctx != "" {
		t.Errorf("expected empty context, got %q", ctx)
	}
}

func TestEngineInjectIntoSystemPrompt(t *testing.T) {
	dir := setupTestRulesDir(t)
	engine := NewEngine([]string{dir})
	engine.LoadRules()

	original := "You are a helpful assistant."
	injected := engine.InjectIntoSystemPrompt(original)

	if !strings.HasPrefix(injected, original) {
		t.Error("expected injected prompt to start with original prompt")
	}
	if !strings.Contains(injected, "--- Rules ---") {
		t.Error("expected injected prompt to contain rules separator")
	}
	if !strings.Contains(injected, "You must always follow this rule.") {
		t.Error("expected injected prompt to contain always-apply rule content")
	}
	// 条件规则不应出现（alwaysApply=false）
	if strings.Contains(injected, "Go Files Rule") {
		t.Error("conditional rule should not be in injected prompt")
	}
}

func TestEngineInjectIntoSystemPromptNoRules(t *testing.T) {
	engine := NewEngine([]string{"/nonexistent"})
	engine.LoadRules()

	original := "You are a helpful assistant."
	injected := engine.InjectIntoSystemPrompt(original)

	if injected != original {
		t.Errorf("expected original prompt unchanged, got %q", injected)
	}
}

func TestEngineRulesPrioritySorted(t *testing.T) {
	dir := setupTestRulesDir(t)
	engine := NewEngine([]string{dir})
	engine.LoadRules()

	applicable := engine.GetApplicableRules()
	if len(applicable) < 2 {
		t.Fatal("expected at least 2 applicable rules")
	}

	// priority-rule (priority=0) 应在 always-rule (priority=1) 之前
	if applicable[0].Priority > applicable[1].Priority {
		t.Errorf("rules not sorted by priority: %d > %d",
			applicable[0].Priority, applicable[1].Priority)
	}
}

func TestEngineReloadRules(t *testing.T) {
	dir := t.TempDir()

	rule1 := `---
alwaysApply: true
---
Rule v1`

	os.WriteFile(filepath.Join(dir, "test.mdc"), []byte(rule1), 0644)

	engine := NewEngine([]string{dir})
	engine.LoadRules()

	if engine.RuleCount() != 1 {
		t.Fatalf("expected 1 rule, got %d", engine.RuleCount())
	}

	// 添加新规则文件
	rule2 := `---
alwaysApply: true
---
Rule v2`
	os.WriteFile(filepath.Join(dir, "test2.mdc"), []byte(rule2), 0644)

	// 重新加载
	engine.LoadRules()
	if engine.RuleCount() != 2 {
		t.Errorf("expected 2 rules after reload, got %d", engine.RuleCount())
	}
}
