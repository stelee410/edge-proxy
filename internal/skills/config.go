package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSkillsFromDirectory 从目录加载所有 Skill 配置文件（.yaml/.yml）
func LoadSkillsFromDirectory(dir string) ([]*SkillConfig, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	var configs []*SkillConfig

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		cfg, err := LoadSkillConfig(path)
		if err != nil {
			return fmt.Errorf("failed to load skill from %q: %w", path, err)
		}

		if cfg.IsEnabled() {
			configs = append(configs, cfg)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return configs, nil
}

// LoadSkillConfig 从单个 YAML 文件加载 Skill 配置
func LoadSkillConfig(path string) (*SkillConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var cfg SkillConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if cfg.Name == "" {
		base := filepath.Base(path)
		cfg.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	if cfg.Stage == "" {
		cfg.Stage = StageMidConversation
	}
	if cfg.Type == "" {
		cfg.Type = TypePromptBased
	}

	return &cfg, nil
}

// ToDefinition 将 SkillConfig 转换为 SkillDefinition
func (c *SkillConfig) ToDefinition() SkillDefinition {
	def := SkillDefinition{
		Name:           c.Name,
		Description:    c.Description,
		DescriptionLLM: c.DescriptionLLM,
		Stage:          c.Stage,
		Type:           c.Type,
		Enabled:        c.IsEnabled(),
	}

	if c.InputSchema != nil {
		data, err := json.Marshal(c.InputSchema)
		if err == nil {
			def.InputSchema = data
		}
	}

	return def
}
