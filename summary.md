# Linkyun Edge Proxy 项目总结

## 项目概述

Linkyun Edge Proxy 是一个基于 Go 语言开发的本地 AI Agent 代理服务，用于在边缘端处理 AI 请求，支持多种大语言模型（LLM）提供商和丰富的扩展机制。

**技术栈**: Go 1.25.6
**代码规模**: 约 9,239 行 Go 代码
**编译产物**: edge-proxy.exe (9.0 MB)

---

## 核心架构

### 目录结构

```
edge-proxy/
├── cmd/                          # 程序入口
│   └── main.go                   # 主程序
├── internal/                     # 核心模块
│   ├── config/                   # 配置管理
│   ├── proxy/                    # 代理服务
│   ├── llm/                      # LLM 提供商
│   ├── skills/                   # Skills 执行管道
│   ├── rules/                    # Rules 规则引擎
│   ├── mcp/                      # MCP 客户端
│   ├── tts/                      # 文本转语音
│   ├── logger/                   # 日志模块
│   └── docparse/                 # 文档解析
├── rules/                        # Rules 定义文件 (.mdc)
├── skills/                       # Skills 定义文件 (.yaml)
├── edge-proxy-config.yaml        # 配置文件
└── edge-proxy-config.yaml.example # 配置示例
```

---

## 核心功能模块

### 1. Proxy 代理服务
**路径**: `internal/proxy/`

- 通过 HTTP 长轮询与 Linkyun Server 通信
- 处理多模态内容（文本、图片、音频、文件）
- Token 用量统计和错误处理
- 支持附件 Base64 编码传输

**核心数据结构**:
- `EdgeRequest`: 服务器请求
- `EdgeResponse`: 代理响应
- `EdgeMessage`: 聊天消息
- `EdgeAttachment`: 附件

### 2. LLM 提供商支持
**路径**: `internal/llm/`

支持 11+ 个 LLM 提供商，包括：

| 提供商 | 类型 | BaseURL |
|--------|------|---------|
| openai | OpenAI | https://api.openai.com/v1 |
| claude | Anthropic | https://api.anthropic.com |
| gemini | Google | https://generativelanguage.googleapis.com |
| ollama | Ollama | http://localhost:11434 |
| deepseek | OpenAI 兼容 | https://api.deepseek.com/v1 |
| qwen | OpenAI 兼容 | https://dashscope.aliyuncs.com |
| doubao | OpenAI 兼容 | https://ark.cn-beijing.volces.com |
| moonshot | OpenAI 兼容 | https://api.moonshot.cn/v1 |
| zhipu | OpenAI 兼容 | https://open.bigmodel.cn |
| ernie | OpenAI 兼容 | https://qianfan.baidubce.com/v2 |

**关键特性**:
- 多 Provider 实例并行运行
- 动态注册和切换
- 降级策略支持
- 预设配置管理

### 3. Skills 执行管道
**路径**: `internal/skills/`

**三阶段执行架构**:
1. **pre_conversation**: 前处理（生成 system prompt）
2. **mid_conversation**: 中间处理（工具调用）
3. **post_conversation**: 后处理（如 TTS）

**三种 Skill 类型**:

| 类型 | 说明 | 示例 |
|------|------|------|
| `prompt-based` | 基于模板 | current-time.yaml（获取当前时间） |
| `prompt-api` | 基于 API | get_weather.yaml（查询天气） |
| `code` | 可执行代码 | voice-tts.yaml（TTS 语音合成） |

**内置 Skills**:
- `current-time.yaml`: 获取当前时间
- `get_weather.yaml`: 查询天气信息
- `voice-tts.yaml`: 文本转语音合成

### 4. Rules 规则引擎
**路径**: `internal/rules/`

**功能**:
- 从 `.mdc` 文件加载规则（Markdown + YAML Frontmatter）
- 支持文件系统监听（热加载）
- 支持 glob 匹配和优先级排序
- 动态添加系统提示词

**Rule 结构**:
```go
type Rule struct {
    Name        string   // 规则名
    Description string   // 描述
    Content     string   // Markdown 内容
    AlwaysApply bool     // 是否始终应用
    Globs       []string // Glob 匹配
    Priority    int      // 优先级
}
```

### 5. MCP (Model Context Protocol) 客户端
**路径**: `internal/mcp/`

**支持的传输方式**:
1. **stdio**: 标准输入输出（进程通信）
2. **sse**: Server-Sent Events（远程 HTTP）

**功能**:
- 文件系统访问（stdio 模式）
- 远程服务集成（SSE 模式）
- 动态资源管理
- JSON-RPC 2.0 通信

**配置示例**:
```yaml
mcp:
  enabled: true
  servers:
    - name: "filesystem"
      transport: "stdio"
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "./docs"]
    - name: "web-search"
      transport: "sse"
      url: "http://localhost:3001/sse"
```

### 6. 其他功能模块

**TTS 文本转语音** (`internal/tts/`)
- 支持 OpenAI、MiniMax 等 TTS 服务
- 可配置语速、音量、语调
- 支持多种情绪和音色

**Logger 日志** (`internal/logger/`)
- 多级日志输出（DEBUG/INFO/WARN/ERROR）
- Token 掩码保护敏感信息

**DocParse 文档解析** (`internal/docparse/`)
- PDF 文档解析（使用 github.com/ledongthuc/pdf）

---

## 配置文件

### 主配置文件: `edge-proxy-config.yaml`

**配置块**:

1. **基础配置**
```yaml
server_url: "http://localhost:8080"    # Linkyun Server
edge_token: "et_..."                   # Edge Token
agent_uuid: "..."                      # Agent UUID
```

2. **LLM 配置**
```yaml
llm:
  default: "openai-gpt4o"              # 默认 Provider
  providers:
    - name: "openai-gpt4o"
      provider: "openai"
      api_key: "sk-xxx"
      model: "gpt-4o-mini"
```

3. **Rules/Skills/MCP 配置**
```yaml
rules:
  enabled: true
  directories: ["./rules"]

skills:
  enabled: true
  directory: "./skills"

mcp:
  enabled: true
  servers: [...]
```

4. **运行参数**
```yaml
heartbeat_interval: 15s                # 心跳间隔
poll_timeout: 30s                      # 轮询超时
log_level: "info"                      # 日志级别
```

---

## 依赖项

**Go 模块** (go.mod):
```
module: linkyun-edge-proxy
go: 1.25.6

依赖:
- gopkg.in/yaml.v3 v3.0.1              # YAML 配置
- github.com/fsnotify/fsnotify v1.9.0  # 文件监听
- github.com/ledongthuc/pdf v0.0.0-... # PDF 解析
```

---

## 代码规模统计

| 模块 | 文件数 | 代码行数 |
|------|--------|---------|
| skills | 16 | ~1,400 |
| llm | 10 | ~1,200 |
| mcp | 10 | ~1,100 |
| rules | 6 | ~800 |
| proxy | 3 | ~600 |
| config | 1 | ~150 |
| **总计** | **约 60** | **~9,239** |

---

## 主要功能

1. **与 Linkyun Server 通信**
   - HTTP 长轮询接收请求
   - 返回 AI 处理结果
   - Token 用量统计

2. **多 LLM 提供商**
   - 支持 11+ 个 LLM 提供商
   - 多实例并行运行
   - 降级策略

3. **Skills 执行管道**
   - 三阶段处理流程
   - 三种 Skill 类型
   - 内置时间、天气、TTS

4. **Rules 规则引擎**
   - 热加载规则文件
   - 条件匹配和优先级
   - 动态系统提示词

5. **MCP 集成**
   - 文件系统访问
   - 远程服务集成
   - 动态资源管理

6. **多模态内容**
   - 文本、图片、音频、文件
   - Base64 编码传输

---

## 适用场景

- 本地 AI Agent 部署
- 边缘计算场景
- 多 LLM 提供商切换和降级
- 自定义 Skills 和 Rules 扩展
- MCP 协议集成
- 文本转语音应用

---

## 总结

Linkyun Edge Proxy 是一个**功能完整、高度可配置的 AI Agent 边缘代理服务**，具备以下特点：

- **多 LLM 支持**: 11+ 个提供商，包括国内外主流模型
- **强大扩展性**: Skills、Rules、MCP 三大扩展机制
- **高性能**: Go 语言实现，9MB 单文件部署
- **易配置**: YAML 配置文件，支持热加载
- **多模态**: 支持文本、图片、音频、文件处理
- **生产就绪**: 日志、监控、错误处理完善

适用于需要本地部署、边缘计算或多 LLM 集成的 AI Agent 应用场景。
