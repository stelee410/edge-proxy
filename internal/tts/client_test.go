package tts

import (
	"testing"
)

func TestExtractTTSText(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTTS      string
		wantClean    string
		wantHasTTS   bool
	}{
		{
			name:       "with tts tag",
			input:      "这是文字回复\n\n<tts>\n你好，这是语音内容。\n</tts>",
			wantTTS:    "你好，这是语音内容。",
			wantClean:  "这是文字回复",
			wantHasTTS: true,
		},
		{
			name:       "no tts tag",
			input:      "这是普通文字回复",
			wantTTS:    "",
			wantClean:  "这是普通文字回复",
			wantHasTTS: false,
		},
		{
			name:       "empty tts tag",
			input:      "文字\n<tts></tts>",
			wantTTS:    "",
			wantClean:  "文字\n<tts></tts>",
			wantHasTTS: false,
		},
		{
			name:       "tts only",
			input:      "<tts>只有语音</tts>",
			wantTTS:    "只有语音",
			wantClean:  "",
			wantHasTTS: true,
		},
		{
			name:       "multiline tts",
			input:      "回复\n<tts>\n第一行\n第二行\n</tts>\n结束",
			wantTTS:    "第一行\n第二行",
			wantClean:  "回复\n\n结束",
			wantHasTTS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttsText, cleanContent, hasTTS := ExtractTTSText(tt.input)
			if hasTTS != tt.wantHasTTS {
				t.Errorf("hasTTS = %v, want %v", hasTTS, tt.wantHasTTS)
			}
			if hasTTS {
				if ttsText != tt.wantTTS {
					t.Errorf("ttsText = %q, want %q", ttsText, tt.wantTTS)
				}
				if cleanContent != tt.wantClean {
					t.Errorf("cleanContent = %q, want %q", cleanContent, tt.wantClean)
				}
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(Config{})
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	// 检查默认值
	if client.config.Provider != "openai" {
		t.Errorf("expected default provider 'openai', got %q", client.config.Provider)
	}
	if client.config.Model != "tts-1" {
		t.Errorf("expected default model 'tts-1', got %q", client.config.Model)
	}
	if client.config.Voice != "alloy" {
		t.Errorf("expected default voice 'alloy', got %q", client.config.Voice)
	}
	if client.config.Format != "mp3" {
		t.Errorf("expected default format 'mp3', got %q", client.config.Format)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model != "tts-1" {
		t.Errorf("expected model 'tts-1', got %q", cfg.Model)
	}
}
