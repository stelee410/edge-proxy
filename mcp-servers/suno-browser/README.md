# Suno Browser MCP Server

通过浏览器自动化方式使用 Suno AI 生成音乐的 MCP Server。

## 功能

- 🌐 自动打开 Suno 网页并显示浏览器
- 🔐 支持手动登录（Google、邮箱等）
- 🎵 自动输入提示词并生成音乐
- 🔗 自动获取分享链接
- 💾 自动保存登录状态

## 环境要求

- Node.js >= 18
- Chromium（Playwright 会在 `npm install` 时自动下载）

## 安装

```bash
cd mcp-servers/suno-browser
npm install
```

## 集成到 edge-proxy

在 `edge-proxy-config.yaml` 中添加：

```yaml
mcp_servers:
  - name: suno
    transport: stdio
    command: node
    args:
      - mcp-servers/suno-browser/index.js
    env:
      NODE_ENV: production
```

## 使用流程

### 1. 首次使用 - 登录

调用 `suno_login` 工具：

```json
{
  "name": "suno_login",
  "arguments": {}
}
```

这将打开浏览器窗口，显示 Suno 网页。请手动完成登录操作。

### 2. 生成音乐

调用 `suno_generate` 工具：

```json
{
  "name": "suno_generate",
  "arguments": {
    "prompt": "一首快乐的流行歌曲，节奏轻快",
    "style": "pop"
  }
}
```

**参数说明：**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| prompt | string | 是 | 音乐描述提示词 |
| style | string | 否 | 音乐风格（pop、rock、electronic、jazz 等）|
| custom_mode | boolean | 否 | 是否使用自定义模式（默认 false）|
| lyrics | string | 否 | 自定义歌词（仅在 custom_mode=true 时使用）|

### 3. 获取分享链接

生成完成后，调用 `suno_get_share_link`：

```json
{
  "name": "suno_get_share_link",
  "arguments": {}
}
```

### 4. 关闭浏览器

使用完毕后，调用 `suno_close`：

```json
{
  "name": "suno_close",
  "arguments": {}
}
```

## 示例对话

```
用户: 帮我生成一首快乐的流行歌曲

AI: 正在为您打开 Suno 页面，请先登录...

[用户在浏览器中完成登录]

AI: 已登录！正在生成音乐...

[等待 30-60 秒]

AI: 🎵 音乐生成完成！分享链接：
https://suno.ai/song/xxx

[点击链接即可播放]
```

## 工具列表

| 工具名 | 说明 |
|--------|------|
| `suno_login` | 打开浏览器，供手动登录 |
| `suno_generate` | 生成音乐 |
| `suno_get_share_link` | 获取分享链接 |
| `suno_close` | 关闭浏览器 |
| `suno_get_page_info` | 获取页面信息（调试用）|
| `suno_inspect_page` | 检查页面元素，帮助诊断选择器问题（调试用）|

## 注意事项

1. **首次使用必须先登录**：调用 `suno_login` 后在浏览器中手动完成登录
2. **登录状态保持**：登录信息会保存在本地，后续使用可自动登录
3. **生成时间**：音乐生成通常需要 30-90 秒
4. **浏览器窗口**：首次使用会显示浏览器窗口，后续可根据需求调整为无头模式
5. **网络要求**：需要能够访问 suno.ai

## 独立测试

可单独测试「打开 Suno 并填写提示词」流程（不依赖 MCP）：

```bash
npm run test:fill-prompt
```

会打开浏览器、跳转到 suno.ai/create，并在 Song Description 输入框填入测试提示词。需事先已登录 Suno。

## 故障排查

### 问题：无法找到输入框

- 检查页面是否完全加载
- 确认已登录
- 尝试使用 `suno_get_page_info` 查看当前页面
- 查看 console 输出中的调试信息，了解发现了哪些可输入元素

### 问题：生成超时

- 检查网络连接
- 确认 Suno 服务是否正常
- 延长等待时间（目前最长 2 分钟）
- 查看 console 输出，了解是否找到生成按钮

### 问题：无法获取分享链接

- 确认音乐已生成完成
- 手动在浏览器中点击分享按钮
- 检查页面 URL 是否变化

### 问题：浏览器启动失败

- 确保 Node.js 版本 >= 18
- 检查防火墙设置
- 尝试删除 `.chrome-data` 文件夹后重试

## 技术栈

- **MCP SDK** - Model Context Protocol
- **Playwright** - 浏览器自动化（支持 :has-text() 等选择器，自动等待元素可操作）
- **Node.js** - 运行环境

## 许可证

MIT
