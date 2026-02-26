package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// 每个 skill 目录下必须有的定义文件名（至少一个）
	skillFileJSON = "SKILL.json"
	skillFileYAML = "SKILL.yaml"
	skillFileYML  = "SKILL.yml"
	skillFileMD   = "SKILL.md"
	// 给人阅读的说明文件
	readmeFile = "README.md"
)

// LoadSkillsFromDirectory 从目录加载所有 Skill。每个 Skill 占一个子目录，子目录内需包含：
// - README.md（给人阅读）
// - 至少一个 SKILL.md 或 SKILL.json（或 SKILL.yaml / SKILL.yml）作为定义文件
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

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var configs []*SkillConfig
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(dir, e.Name())
		cfg, err := loadSkillFromSubdir(subDir, e.Name())
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			continue
		}
		if cfg.IsEnabled() {
			configs = append(configs, cfg)
		}
	}

	return configs, nil
}

// loadSkillFromSubdir 从单个 skill 子目录加载配置。目录内需有 README.md 以及 SKILL.json / SKILL.yaml / SKILL.yml / SKILL.md 之一。
func loadSkillFromSubdir(subDir, dirName string) (*SkillConfig, error) {
	readmePath := filepath.Join(subDir, readmeFile)
	if _, err := os.Stat(readmePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("skill dir %q missing %s (required for human-readable docs)", subDir, readmeFile)
		}
		return nil, fmt.Errorf("skill dir %q: %w", subDir, err)
	}

	// 至少一个定义文件：SKILL.json / SKILL.yaml / SKILL.yml / SKILL.md
	for _, name := range []string{skillFileJSON, skillFileYAML, skillFileYML, skillFileMD} {
		path := filepath.Join(subDir, name)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("skill dir %q: %w", subDir, err)
		}

		cfg, err := LoadSkillConfig(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load skill from %q: %w", path, err)
		}
		if cfg.Name == "" {
			cfg.Name = dirName
		}
		return cfg, nil
	}

	return nil, fmt.Errorf("skill dir %q must have at least one of %s or %s", subDir, skillFileMD, skillFileJSON)
}

// LoadSkillConfig 从单个文件加载 Skill 配置，支持 .json、.yaml/.yml、.md（YAML frontmatter）
func LoadSkillConfig(path string) (*SkillConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var cfg SkillConfig

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	case ".md":
		frontmatter, _ := extractFrontmatter(data)
		if len(frontmatter) == 0 {
			return nil, fmt.Errorf("SKILL.md must contain YAML frontmatter between --- ... ---")
		}
		if err := yaml.Unmarshal(frontmatter, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported skill file extension: %s", ext)
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

// extractFrontmatter 从 Markdown 中提取第一段 YAML frontmatter（--- ... ---）
func extractFrontmatter(data []byte) ([]byte, bool) {
	const delim = "---"
	content := string(data)
	if !strings.HasPrefix(strings.TrimLeft(content, " \t"), delim) {
		return nil, false
	}
	first := strings.Index(content, delim)
	if first < 0 {
		return nil, false
	}
	rest := content[first+len(delim):]
	second := strings.Index(rest, delim)
	if second < 0 {
		return nil, false
	}
	return []byte(strings.TrimSpace(rest[:second])), true
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
