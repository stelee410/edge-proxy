# Voice TTS

检测回复中的 `<tts>` 标签，将文字转为语音并返回音频数据。

- **阶段**: post_conversation  
- **类型**: code (handler: tts)  
- **配置**: 在 SKILL.yaml 的 `config` 中设置 provider（openai / minimax）、model、voice、api_key 等。
