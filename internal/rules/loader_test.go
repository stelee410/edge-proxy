package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoadFromDirectory(t *testing.T) {
	// 创建临时目录和测试文件
	tmpDir := t.TempDir()

	// 创建测试 .mdc 文件
	rule1 := `---
description: Rule One
alwaysApply: true
priority: 2
---

# Rule One Content`

	rule2 := `---
description: Rule Two
alwaysApply: false
globs: "*.go"
priority: 1
---

# Rule Two Content`

	if err := os.WriteFile(filepath.Join(tmpDir, "rule-one.mdc"), []byte(rule1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "rule-two.mdc"), []byte(rule2), 0644); err != nil {
		t.Fatal(err)
	}
	// 非 .mdc 文件应被忽略
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# README"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader([]string{tmpDir})
	ruleSet, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if ruleSet.Count() != 2 {
		t.Fatalf("expected 2 rules, got %d", ruleSet.Count())
	}

	// 验证按优先级排序（priority 1 在前）
	if ruleSet.Rules[0].Priority != 1 {
		t.Errorf("expected first rule priority 1, got %d", ruleSet.Rules[0].Priority)
	}
	if ruleSet.Rules[1].Priority != 2 {
		t.Errorf("expected second rule priority 2, got %d", ruleSet.Rules[1].Priority)
	}
}

func TestLoaderNonExistentDirectory(t *testing.T) {
	loader := NewLoader([]string{"/nonexistent/directory"})
	ruleSet, err := loader.Load()
	if err != nil {
		t.Fatalf("Load should not fail for non-existent dir: %v", err)
	}
	if ruleSet.Count() != 0 {
		t.Errorf("expected 0 rules, got %d", ruleSet.Count())
	}
}

func TestLoaderMultipleDirectories(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	rule1 := `---
alwaysApply: true
---
Rule from dir1`

	rule2 := `---
alwaysApply: false
---
Rule from dir2`

	os.WriteFile(filepath.Join(dir1, "a.mdc"), []byte(rule1), 0644)
	os.WriteFile(filepath.Join(dir2, "b.mdc"), []byte(rule2), 0644)

	loader := NewLoader([]string{dir1, dir2})
	ruleSet, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if ruleSet.Count() != 2 {
		t.Errorf("expected 2 rules, got %d", ruleSet.Count())
	}
}

func TestLoaderRecursiveDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	rule := `---
alwaysApply: true
---
Nested rule`

	os.WriteFile(filepath.Join(subDir, "nested.mdc"), []byte(rule), 0644)

	loader := NewLoader([]string{tmpDir})
	ruleSet, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if ruleSet.Count() != 1 {
		t.Errorf("expected 1 rule from recursive scan, got %d", ruleSet.Count())
	}
}

func TestRuleSetGetAlwaysApply(t *testing.T) {
	rs := NewRuleSet()
	rs.Add(&Rule{Name: "always", AlwaysApply: true})
	rs.Add(&Rule{Name: "conditional", AlwaysApply: false})
	rs.Add(&Rule{Name: "always2", AlwaysApply: true})

	always := rs.GetAlwaysApply()
	if len(always) != 2 {
		t.Errorf("expected 2 always-apply rules, got %d", len(always))
	}
}

func TestRuleSetGetByName(t *testing.T) {
	rs := NewRuleSet()
	rs.Add(&Rule{Name: "test-rule", Description: "test"})

	r := rs.GetByName("test-rule")
	if r == nil {
		t.Fatal("expected to find rule 'test-rule'")
	}
	if r.Description != "test" {
		t.Errorf("unexpected description: %q", r.Description)
	}

	r = rs.GetByName("nonexistent")
	if r != nil {
		t.Error("expected nil for nonexistent rule")
	}
}
