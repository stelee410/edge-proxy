package skills

import (
	"context"
	"fmt"
	"sync"
)

// CodeHandler code 类型 Skill 的执行接口
// 每个 handler 实例在初始化时绑定配置，Execute 时执行逻辑
type CodeHandler interface {
	Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error)
}

// CodeHandlerFactory handler 工厂函数
// config: 来自 YAML 的 handler 配置
// globalCfg: 全局配置（如 LLM 的 api_key、base_url），供 handler 做回退
type CodeHandlerFactory func(config map[string]interface{}, globalCfg map[string]interface{}) (CodeHandler, error)

var (
	handlerFactoriesMu sync.RWMutex
	handlerFactories   = map[string]CodeHandlerFactory{}
)

// RegisterCodeHandler 注册一个 code handler 工厂
func RegisterCodeHandler(name string, factory CodeHandlerFactory) {
	handlerFactoriesMu.Lock()
	defer handlerFactoriesMu.Unlock()
	handlerFactories[name] = factory
}

// CreateCodeHandler 根据名称和配置创建 handler 实例
func CreateCodeHandler(name string, config map[string]interface{}, globalCfg map[string]interface{}) (CodeHandler, error) {
	handlerFactoriesMu.RLock()
	factory, ok := handlerFactories[name]
	handlerFactoriesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown code handler %q (available: %v)", name, ListHandlerNames())
	}

	if config == nil {
		config = make(map[string]interface{})
	}
	if globalCfg == nil {
		globalCfg = make(map[string]interface{})
	}

	return factory(config, globalCfg)
}

// ListHandlerNames 返回所有已注册的 handler 名称
func ListHandlerNames() []string {
	handlerFactoriesMu.RLock()
	defer handlerFactoriesMu.RUnlock()

	names := make([]string, 0, len(handlerFactories))
	for name := range handlerFactories {
		names = append(names, name)
	}
	return names
}

// configString 从 config map 中安全取 string 值，支持 fallback
func configString(config map[string]interface{}, key string, fallback string) string {
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

// configStringFromGlobal 从 config 取值，取不到再从 globalCfg 的对应 key 取
func configStringWithGlobal(config, globalCfg map[string]interface{}, key string, globalKey string, fallback string) string {
	if v := configString(config, key, ""); v != "" {
		return v
	}
	if globalKey != "" {
		if v := configString(globalCfg, globalKey, ""); v != "" {
			return v
		}
	}
	return fallback
}

// configFloat 从 config map 中安全取 float64 值
func configFloat(config map[string]interface{}, key string, fallback float64) float64 {
	if v, ok := config[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return fallback
}

// configInt 从 config map 中安全取 int 值
func configInt(config map[string]interface{}, key string, fallback int) int {
	if v, ok := config[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		case float32:
			return int(n)
		}
	}
	return fallback
}
