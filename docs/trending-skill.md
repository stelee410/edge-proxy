# Trending Skill 使用文档

## 概述

Trending Skill 为 Edge Proxy 提供了获取热门趋势信息的能力，支持从 GitHub 和 Hacker News 获取最新的热门仓库、技术新闻和社区讨论。

## 修复说明

### v1.1.0 (2026-02-25) - 修复 LLM 调用失败问题

**问题**: 当 LLM 调用 trending skill 时，虽然 API 请求成功，但返回原始 JSON 数据导致 LLM 无法正确处理，显示 "execution failed, request failed" 错误。

**修复内容**:
1. 添加 `ResponseTemplate` 字段到 `SkillConfig`
2. 修复 `prompt_api.go` 中的响应模板渲染逻辑
3. 为 YAML skill 添加 `response_template` 配置，格式化 JSON 响应为易读的文本
4. 添加 `addIfNotZero` 和 `len` 模板函数

## 功能特性

### 1. GitHub 趋势 (trending)
- 获取 GitHub 上最热门的开源仓库
- 支持按编程语言过滤
- 支持关键词搜索
- 显示 stars、forks、语言、描述、URL 等信息

### 2. Hacker News 趋势 (trending_hackernews)
- 获取 Hacker News 热门讨论 ID 列表
- 支持多种故事类型（top, new, best）

### 3. 命令行工具 (/trending)
提供更强大的命令行界面，支持：
- 多数据源查询（GitHub, Hacker News, All）
- 时间周期筛选
- 分类筛选
- 详细/简洁格式输出

## 文件结构

```
skills/
├── trending.yaml              # GitHub Trending Skill (LLM 调用)
└── trending-hackernews.yaml   # Hacker News Trending Skill (LLM 调用)

internal/
├── skills/
│   ├── skill.go             # 添加了 ResponseTemplate 字段
│   ├── template.go          # 添加了 addIfNotZero, len 函数
│   └── prompt_api.go        # 修复了响应模板渲染逻辑
└── commands/builtin/
    └── trending.go          # 命令行实现
```

## LLM 调用方式

### 调用 GitHub Trending Skill

当用户询问类似以下问题时，LLM 可以自动调用 `trending` skill：

- "What's trending on GitHub today?"
- "Show me popular repositories"
- "What are the hottest Go projects?"
- "Any interesting AI repositories?"

### 调用 Hacker News Trending Skill

当用户询问类似以下问题时，LLM 可以自动调用 `trending_hackernews` skill：

- "What's popular on Hacker News?"
- "Show me trending tech news"
- "What are people discussing in the tech community?"

## YAML Skill 配置

### trending.yaml

```yaml
name: trending
description: "获取 GitHub 热门仓库信息"
description_for_llm: "Get trending repositories from GitHub..."
stage: mid_conversation
type: prompt-api
input_schema:
  type: object
  properties:
    query:
      type: string
      description: "搜索关键词，如 'machine learning', 'go language', 'AI'"
      default: "is:public"
    language:
      type: string
      description: "编程语言过滤，如 'language:go', 'language:python', 'language:javascript'"
      default: ""
    limit:
      type: integer
      description: "返回结果数量限制"
      default: 10
api_url: "https://api.github.com/search/repositories?q={{.query}}{{addIfNotZero .language}}&sort=stars&order=desc&per_page={{.limit}}"
api_method: GET
response_template: |
  {{range .items}}
  * {{.full_name}}
    {{- if .description}}
    - {{.description}}
    {{- end}}
    Stars: {{.stargazers_count}} | Forks: {{.forks_count}}
    {{- if .language}}
     | Language: {{.language}}
    {{- end}}
    {{if .html_url}}
    URL: {{.html_url}}
    {{end}}

  {{end}}
  Total found: {{.total_count}} repositories
```

## 命令行用法

### 基本命令

```
/trending                # 获取 GitHub 今日热门（默认）
/trend                   # 别名
/hot                     # 别名
```

### 选项参数

| 参数 | 简写 | 说明 | 默认值 |
|-----|------|-----|-------|
| `--source` | `-s` | 数据源：github, hackernews, all | github |
| `--lang` | `-l` | 编程语言（仅 GitHub） | - |
| `--period` | `-p` | 时间周期：daily, weekly, monthly | daily |
| `--category` | `-c` | 分类筛选 | - |
| `--query` | `-q` | 搜索关键词 | - |
| `--limit` | `-n` | 返回数量 | 10 |
| `--format` | `-f` | 输出格式：summary, detailed | summary |

### 使用示例

#### 获取 GitHub 今日热门
```
/trending
```

#### 获取 Go 语言热门仓库
```
/trending -l go
```

#### 获取本周热门
```
/trending -p weekly
```

#### 获取 Hacker News 热门
```
/trending -s hackernews
```

#### 搜索 AI 相关仓库
```
/trending -q "machine learning"
```

#### 获取详细格式输出
```
/trending --format detailed -n 5
```

## 输出示例

### GitHub Trending (LLM 调用结果)

```
* freeCodeCamp/freeCodeCamp
- freeCodeCamp.org's open-source codebase and curriculum.
Stars: 385000 | Forks: 35000 | Language: JavaScript
URL: https://github.com/freeCodeCamp/freeCodeCamp

* facebook/react
- A declarative, efficient, and flexible JavaScript library for building user interfaces.
Stars: 220000 | Forks: 45000 | Language: JavaScript
URL: https://github.com/facebook/react

* tensorflow/tensorflow
- An Open Source Machine Learning Framework for Everyone
Stars: 180000 | Forks: 90000 | Language: Python
URL: https://github.com/tensorflow/tensorflow

Total found: 239311804 repositories
```

## 技术细节

### GitHub API
- 使用 GitHub Search API v3
- 支持高级查询语法
- 自动处理日期范围过滤
- 速率限制处理（10秒超时）

### Hacker News API
- 使用 Hacker News Firebase API
- 实时获取热门故事 ID
- 注意：此 API 仅返回 ID 列表，获取详细信息需要额外请求

### 模板函数
新增的模板函数：

1. **addIfNotZero**: 如果值不为空，返回 "+" + value，否则返回空字符串
   ```yaml
   api_url: "...q={{.query}}{{addIfNotZero .language}}"
   ```

2. **len**: 返回字符串或数组的长度
   ```yaml
   response_template: "Found {{len .}} items"
   ```

### 响应模板
`response_template` 字段允许对 API 返回的 JSON 进行格式化，使 LLM 更容易理解结果。

## 故障排除

### GitHub API 限制
如果遇到速率限制错误：
- 减少 `limit` 参数值
- 等待一段时间后再试

### Hacker News API
- 仅返回故事 ID，不包含详细信息
- 获取完整故事详情需要额外的 API 调用

## 更新日志

### v1.1.0 (2026-02-25)
- 修复 LLM 调用失败问题
- 添加 ResponseTemplate 支持
- 添加 addIfNotZero 和 len 模板函数
- 改进错误处理

### v1.0.0 (2026-02-25)
- 初始版本
- 支持 GitHub Trending API
- 支持 Hacker News API
- 命令行界面
- 两种输出格式
