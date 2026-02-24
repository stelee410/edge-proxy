package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptBasedSkillExecute(t *testing.T) {
	cfg := SkillConfig{
		Name:           "role-enhancer",
		Stage:          StageMidConversation,
		Type:           TypePromptBased,
		PromptTemplate: `你是一个{{.role}}专家。{{if .language}}请使用{{.language}}回答。{{end}}`,
	}

	skill := NewPromptBasedSkill(cfg)

	output, err := skill.Execute(context.Background(), &SkillInput{
		Arguments: map[string]interface{}{
			"role":     "Go编程",
			"language": "中文",
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Success {
		t.Fatalf("expected success, got error: %s", output.Error)
	}
	if !strings.Contains(output.Content, "Go编程专家") {
		t.Errorf("expected content to contain 'Go编程专家', got %q", output.Content)
	}
}

func TestPromptBasedSkillNoTemplate(t *testing.T) {
	cfg := SkillConfig{
		Name:  "empty",
		Stage: StageMidConversation,
		Type:  TypePromptBased,
	}

	skill := NewPromptBasedSkill(cfg)
	_, err := skill.Execute(context.Background(), &SkillInput{})
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestPromptBasedSkillDefinition(t *testing.T) {
	cfg := SkillConfig{
		Name:        "test",
		Description: "Test skill",
		Stage:       StageMidConversation,
		Type:        TypePromptBased,
	}

	skill := NewPromptBasedSkill(cfg)
	def := skill.Definition()

	if def.Name != "test" {
		t.Errorf("expected name 'test', got %q", def.Name)
	}
	if skill.Name() != "test" {
		t.Errorf("expected Name() 'test', got %q", skill.Name())
	}
	if skill.Stage() != StageMidConversation {
		t.Errorf("expected stage 'mid_conversation', got %q", skill.Stage())
	}
	if skill.Type() != TypePromptBased {
		t.Errorf("expected type 'prompt-based', got %q", skill.Type())
	}
}

func TestNewSkillFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      SkillConfig
		wantType string
		wantErr  bool
	}{
		{
			name:     "prompt-based",
			cfg:      SkillConfig{Name: "pb", Type: TypePromptBased, Stage: StageMidConversation},
			wantType: TypePromptBased,
		},
		{
			name:     "prompt-api",
			cfg:      SkillConfig{Name: "pa", Type: TypePromptAPI, Stage: StageMidConversation},
			wantType: TypePromptAPI,
		},
		{
			name:    "unknown type",
			cfg:     SkillConfig{Name: "unk", Type: "unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill, err := NewSkillFromConfig(tt.cfg, nil)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skill.Type() != tt.wantType {
				t.Errorf("expected type %q, got %q", tt.wantType, skill.Type())
			}
		})
	}
}

func TestLoadAndRegisterSkills(t *testing.T) {
	dir := t.TempDir()

	skill1 := `
name: skill1
stage: mid_conversation
type: prompt-based
description: Skill one
prompt_template: "Hello {{.name}}"
`
	skill2 := `
name: skill2
stage: mid_conversation
type: prompt-api
description: Skill two
api_url: "https://example.com/api"
`
	os.WriteFile(filepath.Join(dir, "skill1.yaml"), []byte(skill1), 0644)
	os.WriteFile(filepath.Join(dir, "skill2.yaml"), []byte(skill2), 0644)

	registry := NewRegistry()
	count, err := LoadAndRegisterSkills(dir, registry, nil)
	if err != nil {
		t.Fatalf("LoadAndRegisterSkills failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 skills loaded, got %d", count)
	}
	if registry.Count() != 2 {
		t.Errorf("expected 2 skills in registry, got %d", registry.Count())
	}
}
