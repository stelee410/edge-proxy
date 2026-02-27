# Edge Proxy 下一步行动计划

> 基于 Linkyun 商业计划书和技术文档
> 更新时间：2026-02-25

---

## 核心理解

### Linkyun Edge Proxy 的定位

Edge Proxy 是 Linkyun 平台中的**本地代理服务**，由创作者在本地部署运行：

```
用户消息 → Linkyun 云端 → Redis 队列 → Edge Proxy（创作者本地）
                                                        │
                                            ┌───────────┼───────────┐
                                            │           │           │
                                            ▼           ▼           ▼
                                         Ollama    本地知识库    MCP 工具
                                            │           │           │
                                            └───────────┼───────────┘
                                                        │
                                                        ▼
用户收到回复 ← Linkyun 云端 ← Redis 队列 ← 本地处理结果
```

**关键点**：
- 云端只做**消息路由**，不做推理
- **所有推理在创作者本地完成**
- 支持本地 LLM（Ollama）、本地知识库、MCP 工具
- 满足金融、医疗、法律等行业的**数据合规要求**

### Linkyun Edge Model 的定义

**Linkyun Edge Model 不是某个具体的模型名称，而是指：**

> 创作者在 Edge Proxy 中配置的任何本地大模型接口

常见的本地 LLM 包括：
- Ollama（支持多种开源模型）
- LocalAI
- text-generation-webui
- 其他兼容 OpenAI API 的本地推理服务

---

## 当前 Edge Proxy 的状态

### ✅ 已完成的能力

#### 1. 基础架构（Phase 0 - 初始状态）
- LLM Provider Registry（11+ 提供商支持）
  - OpenAI, Claude, Gemini, DeepSeek, Qwen, Doubao, Moonshot, Zhipu GLM, ERNIE, 通义千问
- Skills Pipeline（预/中/后三阶段）
- Rules Engine（热重载）
- MCP 集成
- TTS 集成
- 基础 TUI

#### 2. LLM Provider 架构
```
internal/llm/
├── provider.go       # Provider 接口定义
├── registry.go      # Provider 注册中心
├── openai.go       # OpenAI 兼容提供商
├── claude.go       # Claude 提供商
├── gemini.go       # Gemini 提供商
├── ollama.go       # Ollama 提供商 ← 这就是本地模型的关键！
└── ...             # 其他提供商
```

**Ollama Provider 已经存在**，支持：
- 通过 HTTP 调用本地运行的 Ollama 服务
- 流式输出
- 多模型切换

### ✅ Phase 1 已完成（本文档之前）

#### 1. 交互式对话模式
- `internal/chat/` 模块完整实现
- 消息结构、上下文管理、会话管理
- 流式响应支持

#### 2. 智能命令系统
- `internal/commands/` 模块完整实现
- 20+ 内置命令
- 命令别名和模糊匹配

#### 3. 增强型 TUI
- 多标签页
- 分屏布局
- 语法高亮
- Markdown 渲染
- 文件浏览器
- 键盘快捷键
- 主题切换

### ⚠️ 当前的缺口

#### 1. 与 Linkyun Server 的连接缺失
- Edge Proxy 目前是独立运行的
- 没有与 Linkyun 云端的通信机制
- 没有 God View（实时提示词注入）的集成

#### 2. 会话状态同步缺失
- 本地会话与云端会话没有关联
- 无法追踪云端会话状态

#### 3. AI 命令未真正工作
- `/ask`、`/explain` 等命令只是占位符
- 没有真正调用 LLM

---

## 下一步行动计划

### 🎯 阶段一：完善本地交互能力（1-2周）

#### 目标
让 Edge Proxy 在**本地模式**下成为一个功能完整的 AI 对话工具，可以：
- 与本地 LLM（Ollama）进行真正的对话
- 管理本地会话
- 支持流式输出
- 支持多模态（图片、文件）

#### 任务清单

| # | 任务 | 优先级 | 预计时间 | 依赖 |
|---|------|---------|-----------|------|
| 1.1 | 完善 LLM 集成，让 ChatClient 真正工作 | P0 | 2天 | - |
| 1.2 | 实现流式输出到 TUI | P0 | 1天 | 1.1 |
| 1.3 | 添加会话持久化（SQLite） | P1 | 2天 | - |
| 1.4 | 完善 AI 命令的实际执行 | P1 | 2天 | 1.1 |
| 1.5 | 添加文件上传和多模态支持 | P2 | 2天 | - |
| 1.6 | 添加会话导出功能 | P2 | 1天 | 1.3 |

#### 详细说明

**1.1 完善 LLM 集成**
- 将 `chat.ChatClient` 与现有的 `llm.Provider` 连接
- 确保可以从 `internal/chat/` 调用 LLM provider
- 实现同步和异步两种模式

**1.2 实现流式输出到 TUI**
- 使用 `chat.StreamHandler` 将流式响应发送到 TUI
- 在 TUI 中实时显示流式输出
- 显示"思考中..."动画

**1.3 添加会话持久化**
```go
// internal/storage/session.go
type SessionStorage struct {
    db *sql.DB
}

// SQLite 表结构
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME,
    updated_at DATETIME,
    config_json TEXT
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME,
    token_count INTEGER,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

**1.4 完善 AI 命令**
- `/ask` - 直接调用 LLM
- `/explain` - 调用 LLM 并添加解释提示
- `/summarize` - 调用 LLM 并添加总结提示
- 等等...

**1.5 文件上传和多模态**
- 在 TUI 中添加文件选择功能
- 支持图片上传（Base64 编码）
- 支持文件读取和分析

**1.6 会话导出**
- `/export markdown` - 导出为 Markdown
- `/export json` - 导出为 JSON
- 支持选择导出范围（最近 N 条消息）

---

### 🚀 阶段二：与 Linkyun Server 集成（2-3周）

#### 目标
让 Edge Proxy 与 Linkyun 云端建立连接，成为 Edge Tunnel 的一部分。

#### 任务清单

| # | 任务 | 优先级 | 预计时间 | 依赖 |
|---|------|---------|-----------|------|
| 2.1 | 设计与 Linkyun Server 的通信协议 | P0 | 1天 | - |
| 2.2 | 实现与云端的长轮询连接 | P0 | 3天 | 2.1 |
| 2.3 | 处理云端推送的用户消息 | P0 | 2天 | 2.2 |
| 2.4 | 实现消息返回给云端 | P0 | 2天 | 2.3, 1.1 |
| 2.5 | 实现 God View 提示词注入 | P1 | 2天 | 2.2 |
| 2.6 | 实现心跳机制 | P1 | 1天 | 2.2 |
| 2.7 | 实现断线重连 | P1 | 1天 | 2.6 |
| 2.8 | 会话状态同步 | P2 | 2天 | 2.3 |

#### 详细说明

**2.1 通信协议设计**
```yaml
# Edge Proxy → Linkyun Server（心跳）
POST /api/v1/edge/heartbeat
Content-Type: application/json

{
  "agent_id": "agent-uuid",
  "proxy_id": "proxy-uuid",
  "status": "online",
  "capabilities": {
    "llm": ["ollama"],
    "knowledge_base": true,
    "mcp_tools": true
  },
  "timestamp": "2026-02-25T10:00:00Z"
}

# Linkyun Server → Edge Proxy（消息推送）
POST /api/v1/edge/message
Content-Type: application/json

{
  "session_id": "session-uuid",
  "message_id": "msg-uuid",
  "user_id": "user-uuid",
  "content": "用户消息",
  "attachments": [],
  "god_view_patch": null  // 创作者的实时提示词注入
}

# Edge Proxy → Linkyun Server（消息响应）
POST /api/v1/edge/response
Content-Type: application/json

{
  "session_id": "session-uuid",
  "message_id": "msg-uuid",
  "role": "assistant",
  "content": "AI 回复",
  "tool_calls": [],
  "token_count": 150,
  "finish_reason": "stop"
}
```

**2.2 实现长轮询**
```go
// internal/server/client.go
type LinkyunClient struct {
    agentID     string
    proxyID     string
    endpoint    string
    httpClient  *http.Client
}

func (c *LinkyunClient) StartPolling(ctx context.Context, msgChan chan<- *IncomingMessage) error {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // 1. 发送心跳
            c.SendHeartbeat()

            // 2. 拉取新消息
            messages, err := c.PollMessages()
            if err != nil {
                log.Error("Poll error:", err)
                continue
            }

            // 3. 将消息发送到处理通道
            for _, msg := range messages {
                msgChan <- msg
            }
        }
    }
}
```

**2.3 处理云端消息**
```go
// 处理队列中的消息
func (p *Proxy) ProcessIncomingMessages(ctx context.Context, msgChan <-chan *IncomingMessage) {
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-msgChan:
            // 1. 获取或创建会话
            session := p.chatManager.GetOrCreateSession(msg.SessionID)

            // 2. 处理 God View 提示词注入
            if msg.GodViewPatch != "" {
                session.SetSystemPrompt(msg.GodViewPatch)
            }

            // 3. 添加用户消息
            session.AddMessage(NewMessage(RoleUser, msg.Content))

            // 4. 调用 LLM
            response, err := p.ProcessMessage(session, msg)
            if err != nil {
                log.Error("Process error:", err)
                // 发送错误回复
                c.SendErrorResponse(msg.MessageID, err.Error())
                continue
            }

            // 5. 发送 AI 回复给云端
            c.SendResponse(msg.MessageID, response)
        }
    }
}
```

**2.4 返回消息给云端**
- 实现 `SendResponse` 方法
- 支持流式响应（如果云端支持）
- 处理超时和重试

**2.5 God View 提示词注入**
```go
// God View 实时更新提示词
func (p *Proxy) HandleGodViewPatch(sessionID, patch string) error {
    session, ok := p.chatManager.GetSession(sessionID)
    if !ok {
        return fmt.Errorf("session not found")
    }

    // 构建最终提示词
    finalPrompt := p.buildFinalPrompt(session, patch)

    // 更新系统提示
    session.SetSystemPrompt(finalPrompt)

    // 记录审计日志
    p.auditLogger.LogPromptChange(sessionID, patch)

    return nil
}
```

**2.6 心跳机制**
- 每 30 秒发送一次心跳
- 云端检测心跳判断 Edge Proxy 在线状态
- 心跳包含：在线状态、能力、版本信息

**2.7 断线重连**
- 检测连接失败
- 指数退避重连（1s, 2s, 4s, 8s, ...）
- 最大重连间隔 60 秒
- 重连成功后恢复正常心跳

**2.8 会话状态同步**
- 将本地会话元数据同步到云端
- 包括：会话名称、创建时间、消息数量
- 支持云端查看会话列表

---

### 🎨 阶段三：增强 TUI 功能（1-2周）

#### 目标
让 TUI 更好地展示本地状态和云端连接状态。

#### 任务清单

| # | 任务 | 优先级 | 预计时间 |
|---|------|---------|-----------|
| 3.1 | 添加云端连接状态显示 | P0 | 1天 |
| 3.2 | 添加 God View 监控视图 | P1 | 2天 |
| 3.3 | 添加消息队列状态 | P2 | 1天 |
| 3.4 | 添加性能统计面板 | P2 | 2天 |

---

### 🔧 阶段四：配置和部署优化（1周）

#### 目标
让 Edge Proxy 更容易配置和部署。

#### 任务清单

| # | 任务 | 优先级 | 预计时间 |
|---|------|---------|-----------|
| 4.1 | 完善 edge-proxy-config.yaml | P0 | 1天 |
| 4.2 | 创建 Docker 部署方案 | P1 | 1天 |
| 4.3 | 创建安装脚本 | P2 | 1天 |
| 4.4 | 编写部署文档 | P2 | 1天 |

#### 配置示例
```yaml
# edge-proxy-config.yaml（完整示例）

# Linkyun 连接配置
linkyun:
  enabled: true
  agent_id: "${LINKYUN_AGENT_ID}"
  agent_api_key: "${LINKYUN_AGENT_API_KEY}"
  server_url: "https://api.linkyun.co"
  polling_interval: 2s  # 长轮询间隔
  heartbeat_interval: 30s  # 心跳间隔

# 本地 LLM 配置
llm:
  default_provider: "ollama"
  providers:
    - name: ollama
      type: ollama
      base_url: "http://localhost:11434"
      model: "llama3.2:3b"
      enabled: true

# 本地知识库配置
knowledge_base:
  enabled: true
  storage_path: "./data/kb"
  chroma_host: "localhost"
  chroma_port: 8000

# MCP 工具配置
mcp:
  enabled: true
  servers:
    - name: filesystem
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/files"]
    - name: brave-search
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-brave-search"]

# 聊天配置
chat:
  enabled: true
  max_messages_per_session: 100
  enable_context_compression: true
  default_system_prompt: "You are a helpful assistant."

# God View 配置
god_view:
  enabled: true
  audit_logging: true
  prompt_update_notification: true
```

---

## 依赖关系图

```
阶段一：本地交互能力
├─ 1.1 完善 LLM 集成
├─ 1.2 流式输出（依赖 1.1）
├─ 1.3 会话持久化
├─ 1.4 AI 命令（依赖 1.1）
├─ 1.5 文件上传
└─ 1.6 会话导出（依赖 1.3）

阶段二：云端集成
├─ 2.1 通信协议设计
├─ 2.2 长轮询（依赖 2.1）
├─ 2.3 处理云端消息（依赖 2.2）
├─ 2.4 返回消息（依赖 2.3, 1.1）
├─ 2.5 God View（依赖 2.2）
├─ 2.6 心跳机制（依赖 2.2）
├─ 2.7 断线重连（依赖 2.6）
└─ 2.8 会话同步（依赖 2.3）

阶段三：TUI 增强
├─ 3.1 连接状态（依赖 2.2）
├─ 3.2 God View 视图（依赖 2.5）
├─ 3.3 消息队列（依赖 2.3）
└─ 3.4 性能统计（依赖 2.3）

阶段四：部署优化
├─ 4.1 配置完善
├─ 4.2 Docker 部署
├─ 4.3 安装脚本
└─ 4.4 部署文档
```

---

## 建议的执行顺序

### 第一优先级：让本地能用起来
**完成阶段一的所有任务**（1-2周）

原因：
- 验证 Edge Proxy 作为本地 AI 工具的核心能力
- 为云端集成打好基础
- 可以开始内部测试和用户反馈收集

### 第二优先级：连接 Linkyun 云端
**完成阶段二的所有任务**（2-3周）

原因：
- 实现核心的 Edge Tunnel 功能
- 支持 God View 实时干预
- 成为 Linkyun 平台完整的一部分

### 第三优先级：体验优化
**完成阶段三的所有任务**（1-2周）

原因：
- 提升用户体验
- 更好地展示 Edge Proxy 能力

### 第四优先级：部署优化
**完成阶段四的所有任务**（1周）

原因：
- 降低用户部署门槛
- 方便创作者快速上手

---

## 关键决策点

### 决策 1：云端通信协议

**选项 A：长轮询**
- 优点：实现简单，兼容性好
- 缺点：延迟较高，浪费资源

**选项 B：WebSocket**
- 优点：实时性好，双向通信
- 缺点：需要处理断线重连

**选项 C：Server-Sent Events (SSE)**
- 优点：标准协议，单向推送
- 缺点：无法从客户端主动发送

**建议**：先用**选项 A（长轮询）**快速实现，后续优化为 WebSocket。

### 决策 2：本地 LLM 配置

**选项 A：只支持 Ollama**
- 优点：简单，文档清晰
- 缺点：限制用户选择

**选项 B：支持多种本地推理服务**
- 优点：灵活，用户可选
- 缺点：复杂度高

**建议**：先支持 Ollama（已有实现），后续添加对其他服务的支持。

### 决策 3：会话存储

**选项 A：纯内存**
- 优点：快速
- 缺点：重启丢失

**选项 B：SQLite**
- 优点：轻量，持久化
- 缺点：需要额外依赖

**建议**：使用 **SQLite**，单文件部署，无需额外配置。

---

## 验收标准

### 阶段一验收
- [ ] 可以与本地 LLM（Ollama）进行对话
- [ ] 支持流式输出
- [ ] 会话可以持久化存储
- [ ] AI 命令可以正常工作
- [ ] 支持文件上传

### 阶段二验收
- [ ] Edge Proxy 可以连接到 Linkyun 云端
- [ ] 可以接收云端推送的用户消息
- [ ] 可以将 AI 回复发送回云端
- [ ] God View 提示词注入可以工作
- [ ] 心跳机制正常工作
- [ ] 断线后可以自动重连

### 阶段三验收
- [ ] TUI 显示云端连接状态
- [ ] TUI 显示 God View 监控信息
- [ ] TUI 显示消息队列状态
- [ ] 性能统计面板正常工作

### 阶段四验收
- [ ] 配置文件完善
- [ ] Docker 部署方案可用
- [ ] 安装脚本可用
- [ ] 部署文档完整

---

## 附录：关键技术点

### A. 长轮询实现参考

```go
// internal/server/poller.go
type Poller struct {
    client       *LinkyunClient
    msgChan      chan<- *IncomingMessage
    lastPollTime time.Time
}

func (p *Poller) Run(ctx context.Context) error {
    ticker := time.NewTicker(p.config.PollingInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // 发送心跳
            if err := p.client.SendHeartbeat(); err != nil {
                log.Error("Heartbeat failed:", err)
            }

            // 拉取消息
            messages, err := p.client.PollMessages(p.lastPollTime)
            if err != nil {
                log.Error("Poll failed:", err)
                continue
            }

            // 处理消息
            for _, msg := range messages {
                p.msgChan <- msg
            }

            p.lastPollTime = time.Now()
        }
    }
}
```

### B. God View 提示词组合

```go
func (p *Proxy) buildFinalPrompt(session *Session, godViewPatch string) string {
    var prompt strings.Builder

    // 基础系统提示
    prompt.WriteString(session.Config.SystemPrompt)

    // 添加 God View 注入
    if godViewPatch != "" {
        prompt.WriteString("\n\n")
        prompt.WriteString("=== CREATOR OVERRIDE ===\n")
        prompt.WriteString(godViewPatch)
        prompt.WriteString("\n=== END OVERRIDE ===\n")
    }

    return prompt.String()
}
```

### C. 断线重连策略

```go
func (c *LinkyunClient) ConnectWithRetry(ctx context.Context) error {
    maxBackoff := 60 * time.Second
    backoff := 1 * time.Second

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            err := c.Connect()
            if err == nil {
                // 连接成功
                backoff = 1 * time.Second // 重置退避
                return nil
            }

            log.Warn("Connect failed, retrying in", backoff)

            // 等待退避时间
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(backoff):
                // 指数退避
                backoff = backoff * 2
                if backoff > maxBackoff {
                    backoff = maxBackoff
                }
            }
        }
    }
}
```

---

*文档版本：1.0*
*最后更新：2026-02-25*
