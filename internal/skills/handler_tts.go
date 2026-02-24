package skills

import (
	"context"
	"fmt"

	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/tts"
)

func init() {
	RegisterCodeHandler("tts", newTTSHandler)
}

// ttsHandler 语音合成 handler
// 检测 post_conversation 内容中的 <tts> 标签，提取文本并调用 TTS API 转语音
type ttsHandler struct {
	client *tts.Client
}

// newTTSHandler 创建 TTS handler
// config 支持: provider, model, voice, format, api_key, base_url
// globalCfg 支持: llm_api_key, llm_base_url（作为 fallback）
func newTTSHandler(config map[string]interface{}, globalCfg map[string]interface{}) (CodeHandler, error) {
	provider := configString(config, "provider", "openai")
	ttsCfg := tts.Config{
		Enabled:  true,
		Provider: provider,
		Model:    configString(config, "model", ""),
		Voice:    configString(config, "voice", ""),
		Format:   configString(config, "format", "mp3"),
		APIKey:   configStringWithGlobal(config, globalCfg, "api_key", "llm_api_key", ""),
		BaseURL:  configStringWithGlobal(config, globalCfg, "base_url", "llm_base_url", ""),
	}

	// MiniMax 专用参数
	if provider == "minimax" {
		ttsCfg.Speed = configFloat(config, "speed", 1.0)
		ttsCfg.Vol = configFloat(config, "vol", 1.0)
		ttsCfg.Pitch = configInt(config, "pitch", 0)
		ttsCfg.Emotion = configString(config, "emotion", "")
	}

	if ttsCfg.APIKey == "" {
		logger.Warn("TTS handler: no api_key configured (set in skill config or global LLM config)")
	}

	client := tts.NewClient(ttsCfg)
	return &ttsHandler{client: client}, nil
}

// Execute 检测 <tts> 标签，转语音，返回清理后的内容 + 音频 metadata
func (h *ttsHandler) Execute(ctx context.Context, input *SkillInput) (*SkillOutput, error) {
	content, _ := input.Arguments["content"].(string)
	if content == "" {
		return &SkillOutput{
			Content: content,
			Success: true,
		}, nil
	}

	ttsText, cleanContent, hasTTS := tts.ExtractTTSText(content)
	if !hasTTS {
		return &SkillOutput{
			Content: content,
			Success: true,
		}, nil
	}
	// 当模型只返回 <tts>xxx</tts> 时，cleanContent 为空，用 ttsText 作为展示文字
	if cleanContent == "" && ttsText != "" {
		cleanContent = ttsText
	}

	logger.Info("TTS handler: detected <tts> tag, converting %d chars to speech", len(ttsText))

	audioResult, err := h.client.Synthesize(ctx, ttsText)
	if err != nil {
		logger.Warn("TTS handler: synthesis failed: %v (returning text only)", err)
		return &SkillOutput{
			Content: content,
			Success: true,
			Error:   fmt.Sprintf("TTS synthesis failed: %v", err),
		}, nil
	}

	return &SkillOutput{
		Content: cleanContent,
		Success: true,
		Metadata: map[string]interface{}{
			"audio_base64": audioResult.AudioBase64,
			"audio_format": audioResult.Format,
		},
	}, nil
}
