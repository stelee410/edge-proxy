package rules

import (
	"testing"
)

func TestParseMDCWithFullFrontmatter(t *testing.T) {
	content := `---
description: 当用户说「继续完成任务」时的开发流程
globs: .cursor/tasks/*.md,.cursor/tasks-done/*.md
alwaysApply: true
priority: 1
---

# 继续完成任务 工作流

当用户说 **「继续完成任务」** 时，按以下流程执行。`

	rule, err := ParseMDC(content, "/rules/continue-task.mdc")
	if err != nil {
		t.Fatalf("ParseMDC failed: %v", err)
	}

	if rule.Name != "continue-task" {
		t.Errorf("expected name 'continue-task', got %q", rule.Name)
	}
	if rule.Description != "当用户说「继续完成任务」时的开发流程" {
		t.Errorf("unexpected description: %q", rule.Description)
	}
	if !rule.AlwaysApply {
		t.Error("expected AlwaysApply=true")
	}
	if rule.Priority != 1 {
		t.Errorf("expected priority 1, got %d", rule.Priority)
	}
	if len(rule.Globs) != 2 {
		t.Fatalf("expected 2 globs, got %d", len(rule.Globs))
	}
	if rule.Globs[0] != ".cursor/tasks/*.md" {
		t.Errorf("unexpected glob[0]: %q", rule.Globs[0])
	}
	if rule.Globs[1] != ".cursor/tasks-done/*.md" {
		t.Errorf("unexpected glob[1]: %q", rule.Globs[1])
	}
	if rule.Content == "" {
		t.Error("expected non-empty content")
	}
	if rule.FilePath != "/rules/continue-task.mdc" {
		t.Errorf("unexpected filepath: %q", rule.FilePath)
	}
}

func TestParseMDCWithoutFrontmatter(t *testing.T) {
	content := `# Simple Rule

This is a rule without frontmatter.`

	rule, err := ParseMDC(content, "/rules/simple.mdc")
	if err != nil {
		t.Fatalf("ParseMDC failed: %v", err)
	}

	if rule.Name != "simple" {
		t.Errorf("expected name 'simple', got %q", rule.Name)
	}
	if rule.Description != "" {
		t.Errorf("expected empty description, got %q", rule.Description)
	}
	if rule.AlwaysApply {
		t.Error("expected AlwaysApply=false")
	}
	if rule.Content != content {
		t.Errorf("unexpected content: %q", rule.Content)
	}
}

func TestParseMDCWithName(t *testing.T) {
	content := `---
name: my-custom-name
description: test
alwaysApply: false
---

Some content.`

	rule, err := ParseMDC(content, "/rules/filename.mdc")
	if err != nil {
		t.Fatalf("ParseMDC failed: %v", err)
	}

	if rule.Name != "my-custom-name" {
		t.Errorf("expected name 'my-custom-name', got %q", rule.Name)
	}
}

func TestParseMDCMinimalFrontmatter(t *testing.T) {
	content := `---
alwaysApply: true
---

Rule content here.`

	rule, err := ParseMDC(content, "/rules/minimal.mdc")
	if err != nil {
		t.Fatalf("ParseMDC failed: %v", err)
	}

	if !rule.AlwaysApply {
		t.Error("expected AlwaysApply=true")
	}
	if rule.Name != "minimal" {
		t.Errorf("expected name 'minimal', got %q", rule.Name)
	}
	if rule.Content != "Rule content here." {
		t.Errorf("unexpected content: %q", rule.Content)
	}
}

func TestParseMDCEmptyContent(t *testing.T) {
	content := `---
description: empty body
alwaysApply: false
---`

	rule, err := ParseMDC(content, "/rules/empty.mdc")
	if err != nil {
		t.Fatalf("ParseMDC failed: %v", err)
	}

	if rule.Content != "" {
		t.Errorf("expected empty content, got %q", rule.Content)
	}
}

func TestParseMDCWithNoFilePath(t *testing.T) {
	content := `---
description: test
---

Content.`

	rule, err := ParseMDC(content, "")
	if err != nil {
		t.Fatalf("ParseMDC failed: %v", err)
	}

	if rule.Name != "" {
		t.Errorf("expected empty name when no filepath, got %q", rule.Name)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectFM   string
		expectBody string
	}{
		{
			name:       "no frontmatter",
			input:      "just body content",
			expectFM:   "",
			expectBody: "just body content",
		},
		{
			name:       "with frontmatter",
			input:      "---\nkey: value\n---\nbody content",
			expectFM:   "key: value",
			expectBody: "body content",
		},
		{
			name:       "only frontmatter",
			input:      "---\nkey: value\n---",
			expectFM:   "key: value",
			expectBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := splitFrontmatter(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm != tt.expectFM {
				t.Errorf("frontmatter: expected %q, got %q", tt.expectFM, fm)
			}
			if body != tt.expectBody {
				t.Errorf("body: expected %q, got %q", tt.expectBody, body)
			}
		})
	}
}
