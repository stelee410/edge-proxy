package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	skill := &mockSkill{name: "test-skill", stage: StageMidConversation, typ: TypePromptBased}
	if err := r.Register(skill); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := r.Get("test-skill")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name() != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", got.Name())
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	r := NewRegistry()

	skill1 := &mockSkill{name: "dup", stage: StageMidConversation, typ: TypePromptBased}
	skill2 := &mockSkill{name: "dup", stage: StagePostConversation, typ: TypePromptAPI}

	if err := r.Register(skill1); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(skill2); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestRegistryGetByStage(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockSkill{name: "s1", stage: StageMidConversation, typ: TypePromptBased})
	r.Register(&mockSkill{name: "s2", stage: StageMidConversation, typ: TypePromptAPI})
	r.Register(&mockSkill{name: "s3", stage: StagePostConversation, typ: TypePromptBased})

	mid := r.GetByStage(StageMidConversation)
	if len(mid) != 2 {
		t.Errorf("expected 2 mid_conversation skills, got %d", len(mid))
	}

	post := r.GetByStage(StagePostConversation)
	if len(post) != 1 {
		t.Errorf("expected 1 post_conversation skill, got %d", len(post))
	}

	pre := r.GetByStage(StagePreConversation)
	if len(pre) != 0 {
		t.Errorf("expected 0 pre_conversation skills, got %d", len(pre))
	}
}

func TestRegistryGetByType(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockSkill{name: "s1", stage: StageMidConversation, typ: TypePromptBased})
	r.Register(&mockSkill{name: "s2", stage: StageMidConversation, typ: TypePromptAPI})
	r.Register(&mockSkill{name: "s3", stage: StagePostConversation, typ: TypePromptAPI})

	pb := r.GetByType(TypePromptBased)
	if len(pb) != 1 {
		t.Errorf("expected 1 prompt-based skill, got %d", len(pb))
	}

	pa := r.GetByType(TypePromptAPI)
	if len(pa) != 2 {
		t.Errorf("expected 2 prompt-api skills, got %d", len(pa))
	}
}

func TestRegistryCount(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Errorf("expected 0, got %d", r.Count())
	}

	r.Register(&mockSkill{name: "s1", stage: StageMidConversation, typ: TypePromptBased})
	if r.Count() != 1 {
		t.Errorf("expected 1, got %d", r.Count())
	}
}

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockSkill{name: "s1", stage: StageMidConversation, typ: TypePromptBased})

	r.Clear()
	if r.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", r.Count())
	}
}

func TestRegistryDefinitions(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockSkill{name: "s1", stage: StageMidConversation, typ: TypePromptBased})
	r.Register(&mockSkill{name: "s2", stage: StagePostConversation, typ: TypePromptAPI})

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}

	midDefs := r.DefinitionsByStage(StageMidConversation)
	if len(midDefs) != 1 {
		t.Errorf("expected 1 mid definition, got %d", len(midDefs))
	}
}

func TestLoadSkillConfig(t *testing.T) {
	dir := t.TempDir()
	skillYAML := `
name: "translate"
stage: "mid_conversation"
type: "prompt-api"
description: "翻译文本到指定语言"
description_for_llm: "Translate text to a specified language"
input_schema:
  type: object
  properties:
    text:
      type: string
      description: "要翻译的文本"
    target_language:
      type: string
      description: "目标语言"
`
	path := filepath.Join(dir, "translate.yaml")
	os.WriteFile(path, []byte(skillYAML), 0644)

	cfg, err := LoadSkillConfig(path)
	if err != nil {
		t.Fatalf("LoadSkillConfig failed: %v", err)
	}

	if cfg.Name != "translate" {
		t.Errorf("expected name 'translate', got %q", cfg.Name)
	}
	if cfg.Stage != StageMidConversation {
		t.Errorf("expected stage 'mid_conversation', got %q", cfg.Stage)
	}
	if cfg.Type != TypePromptAPI {
		t.Errorf("expected type 'prompt-api', got %q", cfg.Type)
	}
	if cfg.Description != "翻译文本到指定语言" {
		t.Errorf("unexpected description: %q", cfg.Description)
	}
	if cfg.InputSchema == nil {
		t.Error("expected non-nil input schema")
	}
}

func TestLoadSkillsFromDirectory(t *testing.T) {
	dir := t.TempDir()

	skill1 := `
name: skill1
stage: mid_conversation
type: prompt-based
description: Skill one
`
	skill2 := `
name: skill2
stage: post_conversation
type: prompt-api
description: Skill two
enabled: false
`
	skill3 := `
name: skill3
stage: pre_conversation
type: prompt-based
description: Skill three
`
	writeSkillSubdir(t, dir, "skill1", skill1)
	writeSkillSubdir(t, dir, "skill2", skill2)
	writeSkillSubdir(t, dir, "skill3", skill3)

	configs, err := LoadSkillsFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadSkillsFromDirectory failed: %v", err)
	}

	// skill2 is disabled, so only 2 configs
	if len(configs) != 2 {
		t.Errorf("expected 2 enabled skills, got %d", len(configs))
	}
}

// writeSkillSubdir 在父目录下创建 skill 子目录，包含 README.md 和 SKILL.yaml
func writeSkillSubdir(t *testing.T, parentDir, subdirName, skillYAML string) {
	t.Helper()
	subdir := filepath.Join(parentDir, subdirName)
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "README.md"), []byte("# "+subdirName), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.yaml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSkillConfigToDefinition(t *testing.T) {
	cfg := &SkillConfig{
		Name:           "test",
		Description:    "test desc",
		DescriptionLLM: "test for llm",
		Stage:          StageMidConversation,
		Type:           TypePromptAPI,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{"type": "string"},
			},
		},
	}

	def := cfg.ToDefinition()
	if def.Name != "test" {
		t.Errorf("expected name 'test', got %q", def.Name)
	}
	if def.InputSchema == nil {
		t.Error("expected non-nil input schema in definition")
	}

	var schema map[string]interface{}
	json.Unmarshal(def.InputSchema, &schema)
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}
}

func TestSkillConfigNameFromFilename(t *testing.T) {
	dir := t.TempDir()
	skill := `
stage: mid_conversation
type: prompt-based
description: No name field
`
	path := filepath.Join(dir, "SKILL.yaml")
	os.WriteFile(path, []byte(skill), 0644)

	cfg, err := LoadSkillConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	// 无 name 时从文件名推断为 "SKILL"，不是 "auto-named"
	if cfg.Name != "SKILL" {
		t.Errorf("expected name 'SKILL' from filename, got %q", cfg.Name)
	}
}
