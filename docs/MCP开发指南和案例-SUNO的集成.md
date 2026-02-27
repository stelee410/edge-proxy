# MCP 开发指南和案例：SUNO 的集成

## 目录

- [一、概述](#一概述)
- [二、MCP 架构原理](#二mcp-架构原理)
- [三、开发环境准备](#三开发环境准备)
- [四、SUNO MCP Server 开发](#四suno-mcp-server-开发)
- [五、集成到 edge-proxy](#五集成到-edge-proxy)
- [六、测试与调试](#六测试与调试)
- [七、最佳实践](#七最佳实践)
- [八、常见问题](#八常见问题)
- [九、附录](#九附录)

---

## 一、概述

### 1.1 什么是 MCP

MCP (Model Context Protocol) 是一种标准协议，用于连接 AI 模型与外部工具和服务。通过 MCP，可以将各种能力（如音乐生成、网页搜索、数据分析等）封装为标准工具，供 AI 模型调用。

### 1.2 为什么选择 MCP

| 特性 | 说明 |
|------|------|
| **标准化** | 统一的协议和接口定义 |
| **语言无关** | 可用任何语言实现（Node.js、Python、Go 等） |
| **热更新** | 修改外部脚本无需重新编译主程序 |
| **解耦** | 业务逻辑独立，易于维护和扩展 |

### 1.3 本指南目标

通过 SUNO 音乐生成 MCP Server 的实际案例，演示如何：
1. 创建一个标准的 MCP Server
2. 将其集成到 edge-proxy
3. 让 AI 模型自动调用该能力

---

## 二、MCP 架构原理

### 2.1 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         edge-proxy                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│   │   LLM       │    │ MCP Manager │    │  Tool       │       │
│   │  (Claude)   │◄──►│             │◄──►│ Executor    │       │
│   └─────────────┘    └─────────────┘    └─────────────┘       │
│         │                   │                  │               │
│         │                   │                  │               │
│         │                   ▼                  │               │
│         │         ┌──────────────────┐        │               │
│         │         │  MCP Server      │        │               │
│         │         │  (stdio/sse)     │        │               │
│         │         └──────────────────┘        │               │
│         │                   │                  │               │
│         │                   ▼                  │               │
│         │         ┌──────────────────┐        │               │
│         └────────►│  Node.js /       │────────┘               │
│                   │  Python Script   │                         │
│                   └──────────────────┘                         │
│                           │                                   │
│                           ▼                                   │
│                   ┌──────────────┐                            │
│                   │  外部 API    │                            │
│                   │  (SUNO 等)   │                            │
│                   └──────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 MCP 消息流程

```
1. 初始化阶段：
   Client ──initialize──► Server
   Client ◄─server_info──┘

2. 工具发现：
   Client ──tools/list──► Server
   Client ◄─tools[]──────┘

3. 工具调用：
   Client ──tools/call──► Server
   Client ◄─result───────┘
```

### 2.3 edge-proxy 中的 MCP 组件

| 文件 | 说明 |
|------|------|
| `internal/mcp/client.go` | MCP 客户端实现 |
| `internal/mcp/manager.go` | MCP 服务器管理器 |
| `internal/mcp/bridge.go` | MCP 工具与 LLM 工具的桥接 |
| `internal/mcp/transport_stdio.go` | stdio 传输层 |
| `internal/mcp/transport_sse.go` | SSE 传输层 |
| `internal/mcp/types.go` | 协议类型定义 |

---

## 三、开发环境准备

### 3.1 系统要求

| 组件 | 版本要求 |
|------|----------|
| Node.js | >= 18 |
| Python | >= 3.9 (如使用 Python) |
| edge-proxy | 最新版本 |

### 3.2 安装 Node.js

```bash
# Windows (使用 nvm-windows)
nvm install 20
nvm use 20

# macOS / Linux
brew install node
# 或使用 nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 20
nvm use 20
```

### 3.3 验证环境

```bash
# 验证 Node.js
node --version
# 应输出: v20.x.x 或更高

# 验证 npm
npm --version
# 应输出: 10.x.x 或更高
```

### 3.4 项目目录结构

```
edge-proxy/
├── edge-proxy-config.yaml     # 主配置文件
├── mcp-servers/               # MCP 服务器目录
│   └── suno/                 # SUNO MCP Server
│       ├── package.json      # 项目配置
│       ├── index.js          # MCP Server 实现
│       ├── README.md         # 说明文档
│       └── node_modules/     # 依赖（自动生成）
```

---

## 四、SUNO MCP Server 开发

### 4.1 创建项目目录

```bash
mkdir -p mcp-servers/suno
cd mcp-servers/suno
```

### 4.2 初始化 package.json

创建 `package.json`：

```json
{
  "name": "suno-mcp-server",
  "version": "1.0.0",
  "description": "MCP Server for Suno AI music generation",
  "type": "module",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "dev": "node --watch index.js"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0.4"
  },
  "engines": {
    "node": ">=18"
  },
  "author": "",
  "license": "MIT"
}
```

### 4.3 安装依赖

```bash
npm install
```

### 4.4 实现 MCP Server

创建 `index.js`：

```javascript
#!/usr/bin/env node

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
} from '@modelcontextprotocol/sdk/types.js';

/**
 * 创建 MCP Server
 */
const server = new Server(
  {
    name: 'suno-mcp-server',
    version: '1.0.0',
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

/**
 * 列出可用工具
 */
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: 'generate_music',
        description: '使用 Suno AI 生成音乐，返回可播放的音频链接',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: '歌曲描述，如「一首快乐的流行歌曲，节奏轻快，有电吉他独奏」',
            },
            style: {
              type: 'string',
              description: '音乐风格，如 pop、rock、electronic、jazz、classical、folk 等',
              enum: ['pop', 'rock', 'electronic', 'jazz', 'classical', 'folk', 'hiphop', 'rnb', 'country', 'blues', 'metal'],
            },
            mood: {
              type: 'string',
              description: '情绪氛围，如 happy、sad、energetic、calm、epic 等',
              enum: ['happy', 'sad', 'energetic', 'calm', 'epic', 'romantic', 'mysterious', 'dramatic'],
            },
            duration: {
              type: 'integer',
              description: '歌曲时长（秒），默认 30，范围 10-120',
              minimum: 10,
              maximum: 120,
            },
            lyrics: {
              type: 'string',
              description: '自定义歌词（可选）',
            },
          },
          required: ['prompt'],
        },
      } satisfies Tool,
      {
        name: 'get_song_status',
        description: '查询歌曲生成状态',
        inputSchema: {
          type: 'object',
          properties: {
            song_id: {
              type: 'string',
              description: '歌曲 ID',
            },
          },
          required: ['song_id'],
        },
      } satisfies Tool,
    ],
  };
});

/**
 * 调用工具
 */
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  switch (name) {
    case 'generate_music':
      return await handleGenerateMusic(args);

    case 'get_song_status':
      return await handleGetSongStatus(args);

    default:
      return {
        content: [
          {
            type: 'text',
            text: `Unknown tool: ${name}`,
          },
        ],
        isError: true,
      };
  }
});

/**
 * 处理音乐生成
 */
async function handleGenerateMusic(args) {
  try {
    const result = await callSunoAPI(args);

    return {
      content: [
        {
          type: 'text',
          text: JSON.stringify({
            success: true,
            song_id: result.song_id,
            title: result.title || `Music: ${args.prompt}`,
            status: result.status,
            audio_url: result.audio_url,
            preview_url: result.preview_url,
            duration: result.duration || args.duration || 30,
            style: args.style,
            mood: args.mood,
          }, null, 2),
        },
      ],
    };
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: JSON.stringify({
            success: false,
            error: error.message,
            details: error.response?.data || null,
          }, null, 2),
        },
      ],
      isError: true,
    };
  }
}

/**
 * 处理状态查询
 */
async function handleGetSongStatus(args) {
  try {
    const result = await callSunoStatusAPI(args.song_id);

    return {
      content: [
        {
          type: 'text',
          text: JSON.stringify(result, null, 2),
        },
      ],
    };
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: JSON.stringify({
            error: error.message,
          }, null, 2),
        },
      ],
      isError: true,
    };
  }
}

/**
 * 调用 Suno 生成 API
 * @param {Object} params - 生成参数
 */
async function callSunoAPI(params) {
  const apiKey = process.env.SUNO_API_KEY;
  if (!apiKey) {
    throw new Error('SUNO_API_KEY environment variable not set');
  }

  // TODO: 根据 Suno 官方 API 文档调整端点和参数
  // 以下是示例实现，需要根据实际 API 调整
  const endpoint = 'https://api.suno.ai/v1/generate';

  const requestBody = {
    prompt: params.prompt,
    style: params.style || 'pop',
    mood: params.mood || 'happy',
    duration: params.duration || 30,
  };

  if (params.lyrics) {
    requestBody.lyrics = params.lyrics;
  }

  const response = await fetch(endpoint, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(requestBody),
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Suno API error: ${response.status} - ${errorText}`);
  }

  const data = await response.json();

  // 返回标准化的结果格式
  return {
    song_id: data.song_id || data.id,
    title: data.title,
    status: data.status || 'completed',
    audio_url: data.audio_url || data.url,
    preview_url: data.preview_url || data.preview_url,
    duration: data.duration || params.duration || 30,
  };
}

/**
 * 调用 Suno 状态查询 API
 * @param {string} songId - 歌曲 ID
 */
async function callSunoStatusAPI(songId) {
  const apiKey = process.env.SUNO_API_KEY;
  if (!apiKey) {
    throw new Error('SUNO_API_KEY environment variable not set');
  }

  const endpoint = `https://api.suno.ai/v1/songs/${songId}`;

  const response = await fetch(endpoint, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${apiKey}`,
    },
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Suno API error: ${response.status} - ${errorText}`);
  }

  return await response.json();
}

/**
 * 启动服务器
 */
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error('Suno MCP Server running on stdio');
}

main().catch((error) => {
  console.error('Fatal error in Suno MCP Server:', error);
  process.exit(1);
});
```

### 4.5 创建 README 文档

创建 `README.md`：

```markdown
# Suno MCP Server

通过 MCP 协议接入 Suno AI 音乐生成能力。

## 功能

- 生成音乐：根据描述、风格、情绪生成音乐
- 查询状态：获取歌曲生成状态和播放链接

## 环境变量

| 变量 | 说明 |
|------|------|
| SUNO_API_KEY | Suno API 密钥（必需） |

## 使用方法

1. 设置 API Key：
   ```bash
   export SUNO_API_KEY="your-api-key-here"
   ```

2. 启动服务：
   ```bash
   npm start
   ```

## 工具列表

### generate_music

生成音乐。

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| prompt | string | 是 | 歌曲描述 |
| style | string | 否 | 音乐风格 |
| mood | string | 否 | 情绪氛围 |
| duration | integer | 否 | 时长（秒）|
| lyrics | string | 否 | 自定义歌词 |

### get_song_status

查询歌曲状态。

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| song_id | string | 是 | 歌曲 ID |

## 示例

```json
{
  "name": "generate_music",
  "arguments": {
    "prompt": "一首快乐的流行歌曲，节奏轻快",
    "style": "pop",
    "mood": "happy",
    "duration": 30
  }
}
```
```

---

## 五、集成到 edge-proxy

### 5.1 修改配置文件

在 `edge-proxy-config.yaml` 中添加 MCP 服务器配置：

```yaml
# MCP 服务器配置
mcp_servers:
  # Suno 音乐生成服务
  - name: suno
    transport: stdio
    command: node
    args:
      - mcp-servers/suno/index.js
    env:
      SUNO_API_KEY: "${SUNO_API_KEY}"  # 从环境变量读取
      NODE_ENV: production
```

### 5.2 设置环境变量

**Linux / macOS:**

```bash
# 临时设置
export SUNO_API_KEY="your-api-key-here"

# 永久设置（添加到 ~/.bashrc 或 ~/.zshrc）
echo 'export SUNO_API_KEY="your-api-key-here"' >> ~/.bashrc
source ~/.bashrc
```

**Windows (PowerShell):**

```powershell
# 临时设置
$env:SUNO_API_KEY = "your-api-key-here"

# 永久设置
[Environment]::SetEnvironmentVariable('SUNO_API_KEY', 'your-api-key-here', 'User')
```

**Windows (CMD):**

```cmd
# 临时设置
set SUNO_API_KEY=your-api-key-here

# 永久设置
setx SUNO_API_KEY "your-api-key-here"
```

### 5.3 启动 edge-proxy

```bash
# 启动服务
./edge-proxy

# 或者带配置文件启动
./edge-proxy -config edge-proxy-config.yaml
```

启动日志应显示：

```
INFO MCP server: suno 1.0.0 (protocol: 2024-11-05)
INFO MCP server suno ready: 2 tools, 0 resources
INFO MCP Manager: 1/1 servers ready
```

---

## 六、测试与调试

### 6.1 直接测试 MCP Server

创建测试脚本 `test.js`：

```javascript
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

async function test() {
  const transport = new StdioClientTransport({
    command: 'node',
    args: ['index.js'],
    env: {
      SUNO_API_KEY: process.env.SUNO_API_KEY,
    },
  });

  const client = new Client({
    name: 'test-client',
    version: '1.0.0',
  }, {
    capabilities: {},
  });

  await client.connect(transport);

  // 列出工具
  const tools = await client.listTools();
  console.log('Available tools:', tools.tools.map(t => t.name));

  // 调用工具
  const result = await client.callTool({
    name: 'generate_music',
    arguments: {
      prompt: '快乐的流行歌曲',
      style: 'pop',
      duration: 30,
    },
  });

  console.log('Result:', result);
}

test().catch(console.error);
```

运行测试：

```bash
node test.js
```

### 6.2 在 edge-proxy 中测试

启动 edge-proxy 后，与 AI 对话：

```
用户: 帮我生成一首快乐的流行歌曲

AI (自动调用 MCP):
  🎵 已为您生成一首歌曲！

  歌曲: Music: 快乐的流行歌曲
  风格: Pop
  时长: 30秒
  状态: 已完成

  [点击播放](https://...)

用户: 再生成一首悲伤的钢琴曲

AI:
  🎵 已为您生成一首歌曲！

  歌曲: Music: 悲伤的钢琴曲
  风格: Classical
  情绪: Sad
  时长: 30秒
  状态: 已完成

  [点击播放](https://...)
```

### 6.3 调试技巧

**查看 MCP Server 日志：**

MCP Server 的 stderr 输出会显示在 edge-proxy 的日志中。

**启用详细日志：**

```javascript
// 在 index.js 中添加调试输出
console.error('DEBUG: Received request:', JSON.stringify(request.params, null, 2));
console.error('DEBUG: Sending result:', JSON.stringify(result, null, 2));
```

**测试 API 连接：**

```bash
# 测试 Suno API 是否可访问
curl -H "Authorization: Bearer $SUNO_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "test", "style": "pop"}' \
  https://api.suno.ai/v1/generate
```

---

## 七、最佳实践

### 7.1 错误处理

```javascript
try {
  const result = await callSunoAPI(params);
  return {
    content: [{ type: 'text', text: JSON.stringify(result) }],
  };
} catch (error) {
  return {
    content: [{ type: 'text', text: JSON.stringify({
      error: error.message,
      type: error.name,
    })}],
    isError: true,
  };
}
```

### 7.2 参数验证

```javascript
function validateGenerateMusicParams(params) {
  if (!params.prompt || typeof params.prompt !== 'string') {
    throw new Error('prompt is required and must be a string');
  }
  if (params.duration && (params.duration < 10 || params.duration > 120)) {
    throw new Error('duration must be between 10 and 120');
  }
}
```

### 7.3 超时控制

```javascript
const TIMEOUT = 60000; // 60秒

async function callSunoAPI(params) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), TIMEOUT);

  try {
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(params),
      signal: controller.signal,
    });
    clearTimeout(timeoutId);
    // ...
  } catch (error) {
    clearTimeout(timeoutId);
    throw error;
  }
}
```

### 7.4 重试机制

```javascript
async function fetchWithRetry(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, options);
      if (response.ok) return response;
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      await new Promise(resolve => setTimeout(resolve, 1000 * (i + 1)));
    }
  }
}
```

### 7.5 日志记录

```javascript
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';

function log(level, message, data) {
  const levels = ['debug', 'info', 'warn', 'error'];
  if (levels.indexOf(level) >= levels.indexOf(LOG_LEVEL)) {
    console.error(`[${level.toUpperCase()}] ${message}`, data ? JSON.stringify(data) : '');
  }
}

// 使用
log('info', 'Starting Suno MCP Server');
log('debug', 'Received request', request.params);
```

---

## 八、常见问题

### Q1: MCP Server 无法启动

**问题：** 启动 edge-proxy 后提示 MCP Server 启动失败

**排查：**
1. 检查 Node.js 版本：`node --version` (需要 >= 18)
2. 检查依赖是否安装：进入 `mcp-servers/suno` 目录运行 `npm install`
3. 检查 API Key 是否设置：`echo $SUNO_API_KEY`

### Q2: 工具调用返回错误

**问题：** AI 调用工具时返回错误

**排查：**
1. 查看 edge-proxy 日志中的 MCP Server 输出
2. 检查 API Key 是否有效
3. 测试 API 连接（见 6.3）
4. 检查参数是否符合 API 要求

### Q3: 传输方式选择

**问题：** 应该用 stdio 还是 sse 传输？

| 传输方式 | 适用场景 |
|----------|----------|
| stdio | 本地运行、简单场景、推荐 |
| sse | 远程服务、需要复用连接 |

大多数情况下，**推荐使用 stdio**。

### Q4: 多个 MCP Server 冲突

**问题：** 有多个 MCP Server 时工具名冲突

**解决：** MCP Manager 会自动添加服务器名前缀：
- Server: `suno`
- Tool: `generate_music`
- 最终工具名: `mcp_suno__generate_music`

### Q5: Python 版本如何实现

**问题：** 想用 Python 而非 Node.js

**解决：** 见附录 A，有完整的 Python 实现示例。

---

## 九、附录

### A. Python 版本实现

**requirements.txt:**

```txt
mcp>=1.0.0
httpx>=0.25.0
```

**server.py:**

```python
#!/usr/bin/env python3
"""
Suno MCP Server - Python 版本
"""

import json
import os
from typing import Any
import httpx

from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import Tool, TextContent

# 创建 Server
server = Server("suno-mcp-server")


@server.list_tools()
async def list_tools() -> list[Tool]:
    """列出可用工具"""
    return [
        Tool(
            name="generate_music",
            description="使用 Suno AI 生成音乐，返回可播放的音频链接",
            inputSchema={
                "type": "object",
                "properties": {
                    "prompt": {
                        "type": "string",
                        "description": "歌曲描述",
                    },
                    "style": {
                        "type": "string",
                        "description": "音乐风格",
                        "enum": ["pop", "rock", "electronic", "jazz", "classical"],
                    },
                    "duration": {
                        "type": "integer",
                        "description": "时长（秒）",
                        "minimum": 10,
                        "maximum": 120,
                    },
                },
                "required": ["prompt"],
            },
        ),
    ]


@server.call_tool()
async def call_tool(name: str, arguments: dict[str, Any]) -> list[TextContent]:
    """调用工具"""
    if name == "generate_music":
        return await generate_music(arguments)

    return [TextContent(type="text", text=f"Unknown tool: {name}")]


async def generate_music(params: dict[str, Any]) -> list[TextContent]:
    """生成音乐"""
    api_key = os.environ.get("SUNO_API_KEY")
    if not api_key:
        return [TextContent(
            type="text",
            text=json.dumps({"error": "SUNO_API_KEY not set"})
        )]

    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                "https://api.suno.ai/v1/generate",
                headers={
                    "Authorization": f"Bearer {api_key}",
                    "Content-Type": "application/json",
                },
                json=params,
                timeout=60.0,
            )
            response.raise_for_status()
            result = response.json()

            return [TextContent(
                type="text",
                text=json.dumps({
                    "success": True,
                    "audio_url": result.get("audio_url"),
                    "title": result.get("title"),
                }, ensure_ascii=False)
            )]
    except Exception as e:
        return [TextContent(
            type="text",
            text=json.dumps({"error": str(e)}, ensure_ascii=False)
        )]


async def main():
    """主函数"""
    async with stdio_server() as (read_stream, write_stream):
        await server.run(read_stream, write_stream)


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
```

**edge-proxy-config.yaml 配置：**

```yaml
mcp_servers:
  - name: suno
    transport: stdio
    command: python
    args:
      - -m
      - venv
      - activate
      - &&
      - python
      - mcp-servers/suno/server.py
    env:
      SUNO_API_KEY: "${SUNO_API_KEY}"
```

### B. MCP 协议参考

**核心方法：**

| 方法 | 说明 |
|------|------|
| `initialize` | 初始化连接 |
| `tools/list` | 获取工具列表 |
| `tools/call` | 调用工具 |
| `resources/list` | 获取资源列表 |
| `resources/read` | 读取资源 |

**文档链接：**
- [MCP 协议规范](https://modelcontextprotocol.io/)
- [MCP SDK - JavaScript](https://github.com/modelcontextprotocol/typescript-sdk)
- [MCP SDK - Python](https://github.com/modelcontextprotocol/python-sdk)

### C. Suno API 参考

**当前文档地址：**
请参考 Suno 官方 API 文档获取最新的端点和参数规范。

**注意：** 本指南中的 Suno API 调用为示例实现，需要根据实际 API 文档进行调整。

---

## 总结

通过本指南，您已经学会了：

1. ✅ 理解 MCP 架构和原理
2. ✅ 创建一个标准的 MCP Server（Node.js/Python）
3. ✅ 将 MCP Server 集成到 edge-proxy
4. ✅ 测试和调试 MCP 工具
5. ✅ 应用最佳实践和解决常见问题

现在您可以基于 SUNO 案例开发更多 MCP Server，扩展 edge-proxy 的能力边界！

---

**文档版本：** 1.0.0
**最后更新：** 2026-02-27
