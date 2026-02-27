package builtin

import (
	"fmt"
	"linkyun-edge-proxy/internal/chat"
	"linkyun-edge-proxy/internal/commands"
	"strings"
	"time"
)

// HistoryCommand 历史记录命令
type HistoryCommand struct {
	manager *chat.Manager
}

// NewHistoryCommand 创建历史记录命令
func NewHistoryCommand(manager *chat.Manager) *HistoryCommand {
	return &HistoryCommand{manager: manager}
}

func (c *HistoryCommand) Name() string              { return "history" }
func (c *HistoryCommand) Description() string       { return "View conversation history" }
func (c *HistoryCommand) Usage() string            { return "/history [limit]" }
func (c *HistoryCommand) Aliases() []string        { return []string{"hist"} }
func (c *HistoryCommand) Category() string        { return "Session" }

func (c *HistoryCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	session, ok := c.manager.GetActiveSession()
	if !ok {
		return "", fmt.Errorf("no active session")
	}

	messages := session.GetMessages()
	if len(args) > 0 {
		limit := 100
		if n, err := fmt.Sscanf(args[0], "%d", &limit); err == nil && n == 1 {
			if limit < len(messages) {
				messages = messages[len(messages)-limit:]
			}
		}
	}

	var sb strings.Builder
	for i, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", i, msg.Role, msg.Content[:50]))
	}
	return sb.String(), nil
}

func (c *HistoryCommand) Validate(args []string) error {
	return nil
}

// ClearCommand 清空对话命令
type ClearCommand struct {
	manager *chat.Manager
}

// NewClearCommand 创建清空命令
func NewClearCommand(manager *chat.Manager) *ClearCommand {
	return &ClearCommand{manager: manager}
}

func (c *ClearCommand) Name() string              { return "clear" }
func (c *ClearCommand) Description() string       { return "Clear current conversation" }
func (c *ClearCommand) Usage() string            { return "/clear" }
func (c *ClearCommand) Aliases() []string        { return []string{} }
func (c *ClearCommand) Category() string        { return "Session" }

func (c *ClearCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	session, ok := c.manager.GetActiveSession()
	if !ok {
		return "", fmt.Errorf("no active session")
	}

	sessionID := session.GetID()
	err := c.manager.ClearSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to clear session: %w", err)
	}

	return fmt.Sprintf("Cleared session: %v", session), nil
}

func (c *ClearCommand) Validate(args []string) error {
	return nil
}

// ResetCommand 重置会话命令
type ResetCommand struct {
	manager *chat.Manager
}

// NewResetCommand 创建重置命令
func NewResetCommand(manager *chat.Manager) *ResetCommand {
	return &ResetCommand{manager: manager}
}

func (c *ResetCommand) Name() string              { return "reset" }
func (c *ResetCommand) Description() string       { return "Reset current session to initial state" }
func (c *ResetCommand) Usage() string            { return "/reset" }
func (c *ResetCommand) Aliases() []string        { return []string{} }
func (c *ResetCommand) Category() string        { return "Session" }

func (c *ResetCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	session, ok := c.manager.GetActiveSession()
	if !ok {
		return "", fmt.Errorf("no active session")
	}

	session.Clear()

	return fmt.Sprintf("Reset session: %v", session), nil
}

func (c *ResetCommand) Validate(args []string) error {
	return nil
}

// SaveSessionCommand 保存会话命令
type SaveSessionCommand struct {
	manager *chat.Manager
}

// NewSaveSessionCommand 创建保存会话命令
func NewSaveSessionCommand(manager *chat.Manager) *SaveSessionCommand {
	return &SaveSessionCommand{manager: manager}
}

func (c *SaveSessionCommand) Name() string              { return "save" }
func (c *SaveSessionCommand) Description() string       { return "Save current session" }
func (c *SaveSessionCommand) Usage() string            { return "/save [name]" }
func (c *SaveSessionCommand) Aliases() []string        { return []string{} }
func (c *SaveSessionCommand) Category() string        { return "Session" }

func (c *SaveSessionCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	name := time.Now().Format("2006-01-02_15-04")
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	session, ok := c.manager.GetActiveSession()
	if !ok {
		return "", fmt.Errorf("no active session")
	}

	session.SetName(name)

	return fmt.Sprintf("Saved session as: %s", name), nil
}

func (c *SaveSessionCommand) Validate(args []string) error {
	return nil
}

// LoadSessionCommand 加载会话命令
type LoadSessionCommand struct {
	manager *chat.Manager
}

// NewLoadSessionCommand 创建加载会话命令
func NewLoadSessionCommand(manager *chat.Manager) *LoadSessionCommand {
	return &LoadSessionCommand{manager: manager}
}

func (c *LoadSessionCommand) Name() string              { return "load" }
func (c *LoadSessionCommand) Description() string       { return "Load a saved session" }
func (c *LoadSessionCommand) Usage() string            { return "/load <name_or_id>" }
func (c *LoadSessionCommand) Aliases() []string        { return []string{} }
func (c *LoadSessionCommand) Category() string        { return "Session" }

func (c *LoadSessionCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("please specify a session name or ID")
	}

	name := args[0]
	session, ok := c.manager.GetSession(name)
	if !ok {
		return "", fmt.Errorf("session not found: %s", name)
	}

	err := c.manager.SetActiveSession(session.GetID())
	if err != nil {
		return "", fmt.Errorf("failed to switch session: %w", err)
	}

	return fmt.Sprintf("Loaded session: %v", session), nil
}

func (c *LoadSessionCommand) Validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session name or ID is required")
	}
	return nil
}

// ListSessionsCommand 列出会话命令
type ListSessionsCommand struct {
	manager *chat.Manager
}

// NewListSessionsCommand 创建列出会话命令
func NewListSessionsCommand(manager *chat.Manager) *ListSessionsCommand {
	return &ListSessionsCommand{manager: manager}
}

func (c *ListSessionsCommand) Name() string              { return "list-sessions" }
func (c *ListSessionsCommand) Description() string       { return "List all sessions" }
func (c *ListSessionsCommand) Usage() string            { return "/list-sessions" }
func (c *ListSessionsCommand) Aliases() []string        { return []string{"sessions", "ls"} }
func (c *ListSessionsCommand) Category() string        { return "Session" }

func (c *ListSessionsCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	sessions := c.manager.ListSessions()

	if len(sessions) == 0 {
		return "No sessions found", nil
	}

	var sb strings.Builder
	sb.WriteString("Sessions:\n\n")
	for _, s := range sessions {
		config := s.GetConfig()
		sb.WriteString(fmt.Sprintf("  %s (Created: %s)\n", config.ID, config.CreatedAt.Format("2006-01-02 15:04")))
		sb.WriteString(fmt.Sprintf("    Model: %s | Messages: %d\n", config.Model, len(s.GetMessages())))
	}
	return sb.String(), nil
}

func (c *ListSessionsCommand) Validate(args []string) error {
	return nil
}

// DeleteSessionCommand 删除会话命令
type DeleteSessionCommand struct {
	manager *chat.Manager
}

// NewDeleteSessionCommand 创建删除会话命令
func NewDeleteSessionCommand(manager *chat.Manager) *DeleteSessionCommand {
	return &DeleteSessionCommand{manager: manager}
}

func (c *DeleteSessionCommand) Name() string              { return "delete-session" }
func (c *DeleteSessionCommand) Description() string       { return "Delete a session" }
func (c *DeleteSessionCommand) Usage() string            { return "/delete-session <id>" }
func (c *DeleteSessionCommand) Aliases() []string        { return []string{"rm", "delete"} }
func (c *DeleteSessionCommand) Category() string        { return "Session" }

func (c *DeleteSessionCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("please specify a session ID")
	}

	id := args[0]
	err := c.manager.DeleteSession(id)
	if err != nil {
		return "", fmt.Errorf("failed to delete session: %w", err)
	}

	return fmt.Sprintf("Deleted session: %s", id), nil
}

func (c *DeleteSessionCommand) Validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session ID is required")
	}
	return nil
}

// RenameSessionCommand 重命名会话命令
type RenameSessionCommand struct {
	manager *chat.Manager
}

// NewRenameSessionCommand 创建重命名会话命令
func NewRenameSessionCommand(manager *chat.Manager) *RenameSessionCommand {
	return &RenameSessionCommand{manager: manager}
}

func (c *RenameSessionCommand) Name() string              { return "rename" }
func (c *RenameSessionCommand) Description() string       { return "Rename a session" }
func (c *RenameSessionCommand) Usage() string            { return "/rename <id> <new_name>" }
func (c *RenameSessionCommand) Aliases() []string        { return []string{} }
func (c *RenameSessionCommand) Category() string        { return "Session" }

func (c *RenameSessionCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("please specify session ID and new name")
	}

	id := args[0]
	name := strings.Join(args[1:], " ")

	err := c.manager.RenameSession(id, name)
	if err != nil {
		return "", fmt.Errorf("failed to rename session: %w", err)
	}

	return fmt.Sprintf("Renamed session %s to: %s", id, name), nil
}

func (c *RenameSessionCommand) Validate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("session ID and new name are required")
	}
	return nil
}

// NewSessionCommand 创建新会话命令
type NewSessionCommand struct {
	manager *chat.Manager
}

// NewNewSessionCommand 创建新会话命令
func NewNewSessionCommand(manager *chat.Manager) *NewSessionCommand {
	return &NewSessionCommand{manager: manager}
}

func (c *NewSessionCommand) Name() string              { return "new" }
func (c *NewSessionCommand) Description() string       { return "Create a new session" }
func (c *NewSessionCommand) Usage() string            { return "/new [name]" }
func (c *NewSessionCommand) Aliases() []string        { return []string{"create", "new-session"} }
func (c *NewSessionCommand) Category() string        { return "Session" }

func (c *NewSessionCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	name := fmt.Sprintf("Session %s", time.Now().Format("15:04"))
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	session, err := c.manager.CreateSession(name)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	err = c.manager.SetActiveSession(session.GetID())
	if err != nil {
		return "", fmt.Errorf("failed to set active session: %w", err)
	}

	return fmt.Sprintf("Created new session: %v", session), nil
}

func (c *NewSessionCommand) Validate(args []string) error {
	return nil
}

// SwitchSessionCommand 切换会话命令
type SwitchSessionCommand struct {
	manager *chat.Manager
}

// NewSwitchSessionCommand 创建切换会话命令
func NewSwitchSessionCommand(manager *chat.Manager) *SwitchSessionCommand {
	return &SwitchSessionCommand{manager: manager}
}

func (c *SwitchSessionCommand) Name() string              { return "switch" }
func (c *SwitchSessionCommand) Description() string       { return "Switch to a different session" }
func (c *SwitchSessionCommand) Usage() string            { return "/switch <id>" }
func (c *SwitchSessionCommand) Aliases() []string        { return []string{"use", "goto"} }
func (c *SwitchSessionCommand) Category() string        { return "Session" }

func (c *SwitchSessionCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	if len(args) == 0 {
		// 列出所有会话
		sessions := c.manager.ListSessions()
		if len(sessions) == 0 {
			return "No sessions available", nil
		}

		var sb strings.Builder
		sb.WriteString("Available sessions:\n\n")
		for _, s := range sessions {
			sb.WriteString(fmt.Sprintf("  %v\n", s.GetID()))
		}
		return sb.String(), nil
	}

	id := args[0]
	err := c.manager.SetActiveSession(id)
	if err != nil {
		return "", fmt.Errorf("failed to switch session: %w", err)
	}

	return fmt.Sprintf("Switched to session: %s", id), nil
}

func (c *SwitchSessionCommand) Validate(args []string) error {
	return nil
}

// ExportCommand 导出命令
type ExportCommand struct {
	manager *chat.Manager
}

// NewExportCommand 创建导出命令
func NewExportCommand(manager *chat.Manager) *ExportCommand {
	return &ExportCommand{manager: manager}
}

func (c *ExportCommand) Name() string              { return "export" }
func (c *ExportCommand) Description() string       { return "Export conversation" }
func (c *ExportCommand) Usage() string            { return "/export [format]" }
func (c *ExportCommand) Aliases() []string        { return []string{} }
func (c *ExportCommand) Category() string        { return "Session" }

func (c *ExportCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	format := "markdown"
	if len(args) > 0 {
		format = args[0]
	}

	session, ok := c.manager.GetActiveSession()
	if !ok {
		return "", fmt.Errorf("no active session")
	}

	messages := session.GetMessages()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Session: %s\n\n", session.GetName()))

	for i, msg := range messages {
		sb.WriteString(fmt.Sprintf("## %d %s\n", i+1, msg.Role))
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", msg.Content))
	}

	return fmt.Sprintf("Exported %d messages as %s\n[Exported file would appear here]", len(messages), format), nil
}

func (c *ExportCommand) Validate(args []string) error {
	return nil
}
