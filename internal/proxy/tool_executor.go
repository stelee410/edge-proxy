package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"linkyun-edge-proxy/internal/llm"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/mcp"
	"linkyun-edge-proxy/internal/sandbox"
	"linkyun-edge-proxy/internal/skills"
)

const (
	maxToolCallingRounds = 10
)

// EdgeReqInfo 从 context 传入的 Edge 请求信息，用于 memory 工具
type EdgeReqInfo struct {
	UserID    string
	AgentUUID string
}

type edgeReqInfoKey struct{}

// memoryActionsKey 用于在 context 中传递 memory 操作收集器
type memoryActionsKey struct{}

// EdgeMemoryAPI Edge 内置 memory 的 API 调用接口
type EdgeMemoryAPI interface {
	SaveMemory(ctx context.Context, userID, agentUUID, content, category string) (string, error)
	DeleteMemoryByKeyword(ctx context.Context, userID, agentUUID, keyword string) (string, error)
}

// ToolExecutor 工具执行器，接收 ToolCall 并查找对应 Skill 或 MCP Tool 执行
type ToolExecutor struct {
	skillRegistry *skills.Registry
	mcpManager    *mcp.Manager
	memoryAPI     EdgeMemoryAPI
	sandbox       sandbox.Executor
}

// NewToolExecutor 创建 Tool 执行器
func NewToolExecutor(registry *skills.Registry) *ToolExecutor {
	return &ToolExecutor{
		skillRegistry: registry,
	}
}

// SetMCPManager 设置 MCP 管理器
func (te *ToolExecutor) SetMCPManager(mgr *mcp.Manager) {
	te.mcpManager = mgr
}

// SetMemoryAPI 设置 Edge Memory API 客户端（用于 save_memory/delete_memory 工具）
func (te *ToolExecutor) SetMemoryAPI(api EdgeMemoryAPI) {
	te.memoryAPI = api
}

// SetSandbox 设置 Bash 沙箱执行器（用于 run_shell 工具）
func (te *ToolExecutor) SetSandbox(sb sandbox.Executor) {
	te.sandbox = sb
}

// Execute 执行一组 tool calls，返回对应的 tool results
func (te *ToolExecutor) Execute(ctx context.Context, toolCalls []llm.ToolCall) []llm.ToolResult {
	results := make([]llm.ToolResult, 0, len(toolCalls))

	for _, tc := range toolCalls {
		result := te.executeOne(ctx, tc)
		results = append(results, result)
	}

	return results
}

// executeOne 执行单个 tool call
// 顺序：save_memory/delete_memory → MCP 工具 → 本地 Skill
func (te *ToolExecutor) executeOne(ctx context.Context, tc llm.ToolCall) llm.ToolResult {
	// 内置 memory 工具（需 memoryAPI 和 context 中的 EdgeReqInfo）
	if tc.Name == "save_memory" || tc.Name == "delete_memory" {
		if te.memoryAPI == nil {
			return llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    "Error: memory API not configured",
				IsError:    true,
			}
		}
		info, _ := ctx.Value(edgeReqInfoKey{}).(*EdgeReqInfo)
		if info == nil || info.UserID == "" || info.AgentUUID == "" {
			return llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    "Error: missing user context for memory operation",
				IsError:    true,
			}
		}
		var content string
		var err error
		if tc.Name == "save_memory" {
			content, err = te.executeSaveMemory(ctx, info, tc.Arguments)
			if err == nil {
				te.appendMemoryAction(ctx, &MemoryAction{Action: "save", Content: getStr(tc.Arguments, "content"), Result: content})
			}
		} else {
			content, err = te.executeDeleteMemory(ctx, info, tc.Arguments)
			if err == nil {
				te.appendMemoryAction(ctx, &MemoryAction{Action: "delete", Keyword: getStr(tc.Arguments, "content_keyword"), Result: content})
			}
		}
		if err != nil {
			return llm.ToolResult{ToolCallID: tc.ID, Content: fmt.Sprintf("Error: %v", err), IsError: true}
		}
		return llm.ToolResult{ToolCallID: tc.ID, Content: content}
	}

	// 内置 run_shell 工具（需沙箱启用）
	if tc.Name == "run_shell" {
		if te.sandbox == nil {
			return llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    "Error: sandbox is not enabled, run_shell is unavailable",
				IsError:    true,
			}
		}
		command := getStr(tc.Arguments, "command")
		if command == "" {
			return llm.ToolResult{ToolCallID: tc.ID, Content: "Error: command is required", IsError: true}
		}
		timeoutSec := 0
		if v, ok := tc.Arguments["timeout_seconds"]; ok {
			switch n := v.(type) {
			case float64:
				timeoutSec = int(n)
			case int:
				timeoutSec = n
			}
		}
		cwd := getStr(tc.Arguments, "cwd")
		var timeout time.Duration
		if timeoutSec > 0 {
			timeout = time.Duration(timeoutSec) * time.Second
		}
		stdout, stderr, exitCode, err := te.sandbox.Run(ctx, command, cwd, timeout)
		if err != nil {
			logger.Warn("run_shell failed: %v", err)
			return llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("Error: %v", err),
				IsError:    true,
			}
		}
		content := stdout
		if stderr != "" {
			content += "\nstderr:\n" + stderr
		}
		content += "\nexit_code: " + strconv.Itoa(exitCode)
		return llm.ToolResult{ToolCallID: tc.ID, Content: content}
	}

	// 检查是否为 MCP 工具
	if mcp.IsMCPTool(tc.Name) {
		if te.mcpManager == nil {
			return llm.ToolResult{
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("Error: MCP is not enabled, cannot execute tool %q", tc.Name),
				IsError:    true,
			}
		}
		logger.Info("Tool call %q (id=%s): executing as MCP tool", tc.Name, tc.ID)
		result := mcp.ExecuteMCPTool(ctx, te.mcpManager, tc.Name, tc.Arguments)
		result.ToolCallID = tc.ID
		return result
	}

	// 本地 Skill 查找
	skill, err := te.skillRegistry.Get(tc.Name)
	if err != nil {
		logger.Warn("Tool call %q (id=%s): skill not found: %v", tc.Name, tc.ID, err)
		return llm.ToolResult{
			ToolCallID: tc.ID,
			Content:    fmt.Sprintf("Error: tool %q not found", tc.Name),
			IsError:    true,
		}
	}

	input := &skills.SkillInput{
		Arguments: tc.Arguments,
	}

	output, err := skill.Execute(ctx, input)
	if err != nil {
		logger.Warn("Tool call %q (id=%s): execution failed: %v", tc.Name, tc.ID, err)
		return llm.ToolResult{
			ToolCallID: tc.ID,
			Content:    fmt.Sprintf("Error executing tool %q: %v", tc.Name, err),
			IsError:    true,
		}
	}

	if !output.Success {
		return llm.ToolResult{
			ToolCallID: tc.ID,
			Content:    output.Error,
			IsError:    true,
		}
	}

	return llm.ToolResult{
		ToolCallID: tc.ID,
		Content:    output.Content,
	}
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (te *ToolExecutor) appendMemoryAction(ctx context.Context, a *MemoryAction) {
	if a == nil {
		return
	}
	collector, _ := ctx.Value(memoryActionsKey{}).(*[]MemoryAction)
	if collector != nil {
		*collector = append(*collector, *a)
	}
}

func (te *ToolExecutor) executeSaveMemory(ctx context.Context, info *EdgeReqInfo, args map[string]interface{}) (string, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return "参数错误：content 不能为空", nil
	}
	if len(content) > 500 {
		content = content[:500]
	}
	category, _ := args["category"].(string)
	if category == "" {
		category = "other"
	}
	msg, err := te.memoryAPI.SaveMemory(ctx, info.UserID, info.AgentUUID, content, category)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func (te *ToolExecutor) executeDeleteMemory(ctx context.Context, info *EdgeReqInfo, args map[string]interface{}) (string, error) {
	keyword, _ := args["content_keyword"].(string)
	if keyword == "" {
		return "参数错误：content_keyword 不能为空", nil
	}
	msg, err := te.memoryAPI.DeleteMemoryByKeyword(ctx, info.UserID, info.AgentUUID, keyword)
	if err != nil {
		return "", err
	}
	return msg, nil
}

// runShellToolDefinition run_shell 工具定义
var runShellToolDefinition = llm.ToolDefinition{
	Name:        "run_shell",
	Description: "Execute a single bash command in a sandbox. Use for file operations, git, npm, listing files, running scripts within the sandbox work directory. Do not run commands that modify system files or require network to download and execute code.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The bash command to run (e.g. 'ls -la', 'git status', 'cat file.txt'). Dangerous commands are blocked by blacklist.",
			},
			"timeout_seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Optional. Max execution time in seconds. If omitted, sandbox default is used.",
			},
			"cwd": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Subdirectory under sandbox work_dir to run the command in.",
			},
		},
		"required": []string{"command"},
	},
}

// memoryToolDefinitions 内置 memory 工具定义
var memoryToolDefinitions = []llm.ToolDefinition{
	{
		Name:        "save_memory",
		Description: "保存一条关于用户的记忆信息。当用户明确要求你记住某些事情（如偏好、习惯、个人信息等），或者对话中出现了值得长期记住的重要用户信息时调用此工具。每条记忆应简洁明确，不超过100字。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "要记住的信息，简洁的一句话描述，如「用户是素食主义者」「用户偏好用 Python 编程」",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"preference", "fact", "constraint", "other"},
					"description": "记忆分类：preference=偏好, fact=事实, constraint=约束限制, other=其他",
				},
			},
			"required": []string{"content"},
		},
	},
	{
		Name:        "delete_memory",
		Description: "删除一条之前保存的用户记忆。当用户要求你忘记某些信息时调用此工具。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content_keyword": map[string]interface{}{
					"type":        "string",
					"description": "要删除的记忆中包含的关键词，用于模糊匹配要删除的记忆",
				},
			},
			"required": []string{"content_keyword"},
		},
	},
}

// GetToolDefinitions 获取所有可用的 tool 定义（本地 Skills + MCP Tools + 可选 memory 工具）
// memoryEnabled 为 true 时追加 save_memory、delete_memory
func (te *ToolExecutor) GetToolDefinitions(memoryEnabled bool) []llm.ToolDefinition {
	var tools []llm.ToolDefinition

	// 本地 Skills（mid_conversation 阶段）
	if te.skillRegistry != nil {
		skillDefs := te.skillRegistry.DefinitionsByStage(skills.StageMidConversation)
		for _, sd := range skillDefs {
			var inputSchema interface{}
			if sd.InputSchema != nil {
				json.Unmarshal(sd.InputSchema, &inputSchema)
			}

			desc := sd.DescriptionLLM
			if desc == "" {
				desc = sd.Description
			}

			tools = append(tools, llm.ToolDefinition{
				Name:        sd.Name,
				Description: desc,
				InputSchema: inputSchema,
			})
		}
	}

	// MCP Tools
	if te.mcpManager != nil {
		mcpTools := mcp.GetAllLLMTools(te.mcpManager)
		tools = append(tools, mcpTools...)
	}

	// 内置 memory 工具
	if memoryEnabled {
		tools = append(tools, memoryToolDefinitions...)
	}

	// 内置 run_shell 工具（沙箱启用时）
	if te.sandbox != nil {
		tools = append(tools, runShellToolDefinition)
	}

	if len(tools) == 0 {
		return nil
	}
	return tools
}
