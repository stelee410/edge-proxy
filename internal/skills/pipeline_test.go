package skills

import (
	"context"
	"fmt"
	"testing"
)

// mockSkill 用于测试的 mock Skill
type mockSkill struct {
	name    string
	stage   string
	typ     string
	output  *SkillOutput
	execErr error
}

func (m *mockSkill) Name() string  { return m.name }
func (m *mockSkill) Stage() string { return m.stage }
func (m *mockSkill) Type() string  { return m.typ }
func (m *mockSkill) Definition() SkillDefinition {
	return SkillDefinition{Name: m.name, Stage: m.stage, Type: m.typ, Enabled: true}
}
func (m *mockSkill) Execute(_ context.Context, _ *SkillInput) (*SkillOutput, error) {
	if m.execErr != nil {
		return &SkillOutput{Success: false, Error: m.execErr.Error()}, m.execErr
	}
	return m.output, nil
}

func TestPipelinePreConversation(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockSkill{
		name:  "context-loader",
		stage: StagePreConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "你是一个专业助手。",
			Success: true,
		},
	})

	pipeline := NewPipeline(registry)
	result, err := pipeline.ExecutePreConversation(context.Background(), &SkillInput{
		UserMessage: "Hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExtraSystemPrompt != "你是一个专业助手。" {
		t.Errorf("expected extra prompt, got %q", result.ExtraSystemPrompt)
	}
}

func TestPipelinePreConversationMultiple(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockSkill{
		name:  "a-context",
		stage: StagePreConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "Context A",
			Success: true,
		},
	})
	registry.Register(&mockSkill{
		name:  "b-context",
		stage: StagePreConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "Context B",
			Success: true,
		},
	})

	pipeline := NewPipeline(registry)
	result, _ := pipeline.ExecutePreConversation(context.Background(), nil)

	if result.ExtraSystemPrompt != "Context A\n\nContext B" {
		t.Errorf("expected combined prompt, got %q", result.ExtraSystemPrompt)
	}
}

func TestPipelinePreConversationSkipOnError(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockSkill{
		name:    "failing-skill",
		stage:   StagePreConversation,
		typ:     TypePromptBased,
		execErr: errSkillFailed,
	})
	registry.Register(&mockSkill{
		name:  "working-skill",
		stage: StagePreConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "Works fine",
			Success: true,
		},
	})

	pipeline := NewPipeline(registry)
	result, err := pipeline.ExecutePreConversation(context.Background(), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExtraSystemPrompt != "Works fine" {
		t.Errorf("expected only working skill output, got %q", result.ExtraSystemPrompt)
	}
}

func TestPipelinePostConversation(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockSkill{
		name:  "formatter",
		stage: StagePostConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "Formatted: hello world",
			Success: true,
		},
	})

	pipeline := NewPipeline(registry)
	result, err := pipeline.ExecutePostConversation(context.Background(), "hello world", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "Formatted: hello world" {
		t.Errorf("expected formatted content, got %q", result.Content)
	}
}

func TestPipelinePostConversationChain(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockSkill{
		name:  "a-post",
		stage: StagePostConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "Step A processed",
			Success: true,
		},
	})
	registry.Register(&mockSkill{
		name:  "b-post",
		stage: StagePostConversation,
		typ:   TypePromptBased,
		output: &SkillOutput{
			Content: "Step B processed",
			Success: true,
		},
	})

	pipeline := NewPipeline(registry)
	result, _ := pipeline.ExecutePostConversation(context.Background(), "original", nil)

	if result.Content != "Step B processed" {
		t.Errorf("expected final post result, got %q", result.Content)
	}
}

func TestPipelinePostConversationEmpty(t *testing.T) {
	registry := NewRegistry()
	pipeline := NewPipeline(registry)

	result, err := pipeline.ExecutePostConversation(context.Background(), "original content", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "original content" {
		t.Errorf("expected original content, got %q", result.Content)
	}
}

func TestPipelineGetMidConversationDefinitions(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockSkill{
		name:  "tool1",
		stage: StageMidConversation,
		typ:   TypePromptBased,
	})
	registry.Register(&mockSkill{
		name:  "pre-skill",
		stage: StagePreConversation,
		typ:   TypePromptBased,
	})

	pipeline := NewPipeline(registry)
	defs := pipeline.GetMidConversationDefinitions()

	if len(defs) != 1 {
		t.Fatalf("expected 1 mid-conversation definition, got %d", len(defs))
	}
	if defs[0].Name != "tool1" {
		t.Errorf("expected tool1, got %q", defs[0].Name)
	}
}

var errSkillFailed = fmt.Errorf("skill execution failed")
