package skills

import (
	"fmt"
	"sync"
)

// Registry Skill 注册中心，管理所有可用的本地 Skills
type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill // name -> Skill
}

// NewRegistry 创建 Skill 注册中心
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// Register 注册一个 Skill
func (r *Registry) Register(skill Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := skill.Name()
	if _, exists := r.skills[name]; exists {
		return fmt.Errorf("skill %q already registered", name)
	}
	r.skills[name] = skill
	return nil
}

// Get 根据名称获取 Skill
func (r *Registry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return skill, nil
}

// GetByStage 获取指定阶段的所有 Skill
func (r *Registry) GetByStage(stage string) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Skill
	for _, skill := range r.skills {
		if skill.Stage() == stage {
			result = append(result, skill)
		}
	}
	return result
}

// GetByType 获取指定类型的所有 Skill
func (r *Registry) GetByType(skillType string) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Skill
	for _, skill := range r.skills {
		if skill.Type() == skillType {
			result = append(result, skill)
		}
	}
	return result
}

// All 返回所有已注册的 Skill
func (r *Registry) All() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		result = append(result, skill)
	}
	return result
}

// Names 返回所有已注册 Skill 的名称
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

// Count 返回已注册 Skill 的数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// Clear 清空所有注册的 Skill
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = make(map[string]Skill)
}

// Definitions 返回所有 Skill 的定义（用于 LLM tool calling）
func (r *Registry) Definitions() []SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]SkillDefinition, 0, len(r.skills))
	for _, skill := range r.skills {
		defs = append(defs, skill.Definition())
	}
	return defs
}

// DefinitionsByStage 返回指定阶段的 Skill 定义
func (r *Registry) DefinitionsByStage(stage string) []SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []SkillDefinition
	for _, skill := range r.skills {
		if skill.Stage() == stage {
			defs = append(defs, skill.Definition())
		}
	}
	return defs
}
