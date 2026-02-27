# Skills 目录

每个 Skill 占一个**子目录**，目录内需包含：

- **README.md** — 给人阅读的说明（必填）
- **至少一个定义文件**（必填）：
  - `SKILL.json` 或
  - `SKILL.yaml` / `SKILL.yml` 或
  - `SKILL.md`（需含 YAML frontmatter）

加载时只会扫描一级子目录；每个子目录中若缺少 README.md 或缺少任一 SKILL 定义文件，该目录会被跳过或报错。

## 当前 Skills

| 目录 | 说明 |
|------|------|
| `trending/` | GitHub 热门仓库 |
| `trending_hackernews/` | Hacker News 热门讨论 |
| `get_weather/` | 城市天气查询 |
| `voice-tts/` | TTS 语音合成 |
| `current-time/` | 当前时间 |
| `web_search/` | 网络搜索并整理成文字报告 |

详见各子目录下的 README.md。
