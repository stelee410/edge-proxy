# Command Reference

This document provides a comprehensive reference for all available commands in Linkyun Edge Proxy.

## Usage

Commands are invoked by prefixing them with `/`. For example:
```
/help
/ask What is the weather today?
/model gpt-4
```

## Command Categories

### General Commands

#### `/help [category|command]`
Display help information.

**Aliases**: `h`, `?`

**Examples**:
```
/help                    # Show all command categories
/help General            # Show commands in General category
/help ask                # Show detailed help for ask command
```

---

### Session Commands

#### `/history [limit]`
View conversation history for the current session.

**Aliases**: `hist`

**Examples**:
```
/history          # Show recent history
/history 50      # Show last 50 messages
```

#### `/clear`
Clear the current conversation (keeps system prompt).

**Example**:
```
/clear
```

#### `/reset`
Reset the current session to initial state (removes all messages including system prompt).

**Example**:
```
/reset
```

#### `/save [name]`
Save the current session.

**Example**:
```
/save                  # Save with default name
/save My Conversation   # Save with custom name
```

#### `/load <name_or_id>`
Load a saved session.

**Example**:
```
/load My Conversation
/load session_20240225_123456
```

#### `/list-sessions`
List all sessions.

**Aliases**: `sessions`, `ls`

**Example**:
```
/list-sessions
```

#### `/delete-session <id>`
Delete a session.

**Aliases**: `rm`, `delete`

**Example**:
```
/delete-session session_20240225_123456
```

#### `/rename <id> <new_name>`
Rename a session.

**Example**:
```
/rename session_123456 "New Name"
```

#### `/new [name]`
Create a new session.

**Aliases**: `create`, `new-session`

**Example**:
```
/new                  # Create with default name
/new "My Project"     # Create with custom name
```

#### `/switch [id]`
Switch to a different session. Without arguments, lists available sessions.

**Aliases**: `use`, `goto`

**Examples**:
```
/switch                # List sessions
/switch session_123456  # Switch to specific session
```

#### `/export [format]`
Export the conversation.

**Supported formats**: `markdown`, `html`, `json`, `txt`

**Example**:
```
/export markdown
/export html
```

---

### Configuration Commands

#### `/model [model_name]`
Set or view the current model.

**Example**:
```
/model           # View current model
/model gpt-4    # Set model to gpt-4
/model claude-3  # Set model to claude-3
```

#### `/temperature <value>`
Set the temperature parameter (0.0 - 2.0). Higher values make output more random.

**Aliases**: `temp`

**Examples**:
```
/temperature 0.7    # Balanced
/temperature 0.2    # More focused
/temperature 1.5    # More creative
```

#### `/system <prompt>`
Set the system prompt that guides the AI's behavior.

**Aliases**: `sys`

**Example**:
```
/system You are a helpful coding assistant who writes clean, well-documented code.
```

#### `/settings`
View or edit configuration settings.

**Aliases**: `config`, `cfg`

**Example**:
```
/settings
```

---

### AI Commands

#### `/ask <question>`
Ask a question to the AI.

**Aliases**: `a`, `?`

**Example**:
```
/ask What is the difference between Go and Rust?
```

#### `/summarize <file|text>`
Summarize a file or text.

**Aliases**: `sum`

**Examples**:
```
/summarize README.md
/summarize "This is a long text that needs summarizing"
```

#### `/explain <file|code|text>`
Explain code or text.

**Aliases**: `exp`

**Examples**:
```
/explain main.go
/explain "for i := 0; i < 10; i++ { println(i) }"
```

#### `/refactor <file>`
Refactor code in a file.

**Example**:
```
/refactor old_code.go
```

#### `/review [files...]`
Review code in one or more files.

**Example**:
```
/review main.go
/review *.go
/review app.js utils.js
```

#### `/fix <error|file:line>`
Fix an error or bug.

**Example**:
```
/fix "undefined variable 'x'"
/fix main.go:42
```

#### `/test <file>`
Generate tests for code.

**Example**:
```
/test main.go
```

#### `/optimize <file>`
Optimize code for performance.

**Aliases**: `opt`

**Example**:
```
/optimize database.go
```

#### `/document <file>`
Generate documentation for code.

**Aliases**: `doc`

**Example**:
```
/document api.go
```

#### `/translate <text> [to_lang]`
Translate text to another language.

**Aliases**: `tr`

**Examples**:
```
/translate "Hello, world!" Spanish
/translate "Bonjour" English
```

#### `/analyze <file>`
Perform deep analysis on code or a file.

**Aliases**: `anal`

**Example**:
```
/analyze main.go
```

#### `/diff <file1> <file2>`
Compare two files.

**Example**:
```
/diff original.go modified.go
```

---

## Keyboard Shortcuts

### Global
- `Ctrl+C` - Quit
- `F1` - Show keybindings help

### Navigation
- `↑` / `Ctrl+P` - Navigate up
- `↓` / `Ctrl+N` - Navigate down
- `←` / `Ctrl+B` - Navigate left
- `→` / `Ctrl+F` - Navigate right
- `PgUp` - Page up
- `PgDn` - Page down
- `Ctrl+U` - Scroll up half page
- `Ctrl+D` - Scroll down half page
- `Home` / `Ctrl+A` - Go to beginning
- `End` / `Ctrl+E` - Go to end

### Tabs
- `Ctrl+→` - Next tab
- `Ctrl+←` - Previous tab
- `Ctrl+T` - New tab
- `Ctrl+W` - Close tab

### Editing
- `Ctrl+S` - Save
- `Ctrl+R` - Refresh
- `Ctrl+K` - Delete line
- `Ctrl+W` - Delete word

---

## Configuration

Command behavior can be customized in `edge-proxy-config.yaml`:

```yaml
# Command configuration
commands:
  enabled: true
  prefix: "/"
  aliases:
    ask: ["a", "?"]
    help: ["h"]
  fuzzy_match: true
```

---

## Tips

1. **Auto-completion**: Press `Tab` to auto-complete command names.
2. **History**: Use `↑` and `↓` to navigate command history.
3. **Help**: Use `/help` to see available commands or `/help <command>` for detailed information.
4. **Fuzzy matching**: Typing partial command names will attempt to find matches (e.g., `/his` → `/history`).
