package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

var ttsTagRegex = regexp.MustCompile(`(?s)<tts>(.*?)</tts>`)

// Config TTS 配置
type Config struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"` // "openai" 或 "minimax"
	BaseURL  string `yaml:"base_url"` // API 地址，默认用 OpenAI
	APIKey   string `yaml:"api_key"`  // API Key，为空时复用 LLM 的 key
	Model    string `yaml:"model"`    // TTS 模型，默认 "tts-1"（OpenAI）或 "speech-2.8-hd"（MiniMax）
	Voice    string `yaml:"voice"`    // 语音角色/音色 ID
	Format   string `yaml:"format"`   // 输出格式，默认 "mp3"

	// MiniMax 专用配置
	Speed   float64 `yaml:"speed"`   // 语速 [0.5, 2]，默认 1
	Vol     float64 `yaml:"vol"`     // 音量 (0, 10]，默认 1
	Pitch   int     `yaml:"pitch"`   // 语调 [-12, 12]，默认 0
	Emotion string  `yaml:"emotion"` // 情绪控制
}

// DefaultConfig 返回默认 TTS 配置
func DefaultConfig() Config {
	return Config{
		Provider: "openai",
		Model:    "tts-1",
		Voice:    "alloy",
		Format:   "mp3",
	}
}

// Client TTS 客户端
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient 创建 TTS 客户端
func NewClient(cfg Config) *Client {
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}
	if cfg.Model == "" {
		cfg.Model = "tts-1"
	}
	if cfg.Voice == "" {
		cfg.Voice = "alloy"
	}
	if cfg.Format == "" {
		cfg.Format = "mp3"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// AudioResult TTS 转换结果
type AudioResult struct {
	AudioBase64 string `json:"audio_base64"` // Base64 编码的音频数据
	Format      string `json:"format"`       // 音频格式 (mp3, wav 等)
	Text        string `json:"text"`         // 原始文本
}

// ExtractTTSText 从内容中提取 <tts>...</tts> 标签中的文本
// 返回提取的文本和去掉 <tts> 标签后的内容
func ExtractTTSText(content string) (ttsText string, cleanContent string, hasTTS bool) {
	matches := ttsTagRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return "", content, false
	}

	ttsText = strings.TrimSpace(matches[1])
	if ttsText == "" {
		return "", content, false
	}

	cleanContent = ttsTagRegex.ReplaceAllString(content, "")
	cleanContent = strings.TrimSpace(cleanContent)

	return ttsText, cleanContent, true
}

// Synthesize 文字转语音
func (c *Client) Synthesize(ctx context.Context, text string) (*AudioResult, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	switch c.config.Provider {
	case "openai":
		return c.synthesizeOpenAI(ctx, text)
	case "minimax":
		return c.synthesizeMinimax(ctx, text)
	default:
		return nil, fmt.Errorf("unsupported TTS provider: %q", c.config.Provider)
	}
}

// synthesizeOpenAI 调用 OpenAI TTS API
func (c *Client) synthesizeOpenAI(ctx context.Context, text string) (*AudioResult, error) {
	reqBody := map[string]interface{}{
		"model":           c.config.Model,
		"input":           text,
		"voice":           c.config.Voice,
		"response_format": c.config.Format,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TTS request: %w", err)
	}

	url := strings.TrimSuffix(c.config.BaseURL, "/") + "/audio/speech"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	logger.Debug("TTS request: text length=%d, voice=%s, model=%s", len(text), c.config.Voice, c.config.Model)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS API returned status %d: %s", resp.StatusCode, string(audioData))
	}

	audioBase64 := base64.StdEncoding.EncodeToString(audioData)

	logger.Info("TTS completed: text=%d chars, audio=%d bytes, format=%s",
		len(text), len(audioData), c.config.Format)

	return &AudioResult{
		AudioBase64: audioBase64,
		Format:      c.config.Format,
		Text:        text,
	}, nil
}

// synthesizeMinimax 调用 MiniMax T2A v2 API
func (c *Client) synthesizeMinimax(ctx context.Context, text string) (*AudioResult, error) {
	type voiceSetting struct {
		VoiceID string  `json:"voice_id"`
		Speed   float64 `json:"speed,omitempty"`
		Vol     float64 `json:"vol,omitempty"`
		Pitch   int     `json:"pitch,omitempty"`
		Emotion string  `json:"emotion,omitempty"`
	}
	type audioSetting struct {
		SampleRate int    `json:"sample_rate,omitempty"`
		Bitrate    int    `json:"bitrate,omitempty"`
		Format     string `json:"format,omitempty"`
		Channel    int    `json:"channel,omitempty"`
	}
	type t2aReq struct {
		Model        string        `json:"model"`
		Text         string        `json:"text"`
		Stream       bool          `json:"stream"`
		VoiceSetting *voiceSetting `json:"voice_setting,omitempty"`
		AudioSetting *audioSetting `json:"audio_setting,omitempty"`
		OutputFormat string        `json:"output_format,omitempty"`
	}

	model := c.config.Model
	if model == "" {
		model = "speech-2.8-hd"
	}
	voice := c.config.Voice
	if voice == "" {
		voice = "male-qn-qingse"
	}
	speed := c.config.Speed
	if speed == 0 {
		speed = 1.0
	}
	vol := c.config.Vol
	if vol == 0 {
		vol = 1.0
	}
	format := c.config.Format
	if format == "" {
		format = "mp3"
	}

	reqBody := t2aReq{
		Model:        model,
		Text:         text,
		Stream:       false,
		OutputFormat: "hex",
		VoiceSetting: &voiceSetting{
			VoiceID: voice,
			Speed:   speed,
			Vol:     vol,
			Pitch:   c.config.Pitch,
			Emotion: c.config.Emotion,
		},
		AudioSetting: &audioSetting{
			SampleRate: 32000,
			Bitrate:    128000,
			Format:     format,
			Channel:    1,
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MiniMax TTS request: %w", err)
	}

	apiURL := "https://api.minimaxi.com/v1/t2a_v2"
	if c.config.BaseURL != "" {
		apiURL = strings.TrimSuffix(c.config.BaseURL, "/") + "/v1/t2a_v2"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create MiniMax TTS request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	logger.Debug("MiniMax TTS request: text length=%d, voice=%s, model=%s", len(text), voice, model)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("MiniMax TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read MiniMax TTS response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MiniMax TTS API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	type t2aResp struct {
		Data *struct {
			Audio  string `json:"audio"`
			Status int    `json:"status"`
		} `json:"data"`
		ExtraInfo *struct {
			AudioLength int64  `json:"audio_length"`
			AudioSize   int64  `json:"audio_size"`
			AudioFormat string `json:"audio_format"`
			UsageChars  int64  `json:"usage_characters"`
		} `json:"extra_info"`
		TraceID  string `json:"trace_id"`
		BaseResp *struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}

	var result t2aResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode MiniMax TTS response: %w", err)
	}

	if result.BaseResp != nil && result.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("MiniMax TTS API error %d: %s (trace_id: %s)",
			result.BaseResp.StatusCode, result.BaseResp.StatusMsg, result.TraceID)
	}
	if result.Data == nil || result.Data.Audio == "" {
		return nil, fmt.Errorf("MiniMax TTS returned empty audio (trace_id: %s)", result.TraceID)
	}

	// MiniMax 返回 hex 编码，需要解码为 bytes 再 base64
	audioBytes, err := hex.DecodeString(result.Data.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MiniMax TTS hex audio: %w", err)
	}
	audioBase64 := base64.StdEncoding.EncodeToString(audioBytes)

	if result.ExtraInfo != nil {
		logger.Info("MiniMax TTS completed: text=%d chars, audio=%d bytes (%s), duration=%dms",
			result.ExtraInfo.UsageChars, result.ExtraInfo.AudioSize,
			result.ExtraInfo.AudioFormat, result.ExtraInfo.AudioLength)
	}

	return &AudioResult{
		AudioBase64: audioBase64,
		Format:      format,
		Text:        text,
	}, nil
}
