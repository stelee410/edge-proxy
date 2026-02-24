package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"linkyun-edge-proxy/internal/llm"
)

const (
	// MCPToolPrefix MCP 工具名称前缀
	MCPToolPrefix = "mcp_"
)

// MCPToolsToLLMTools 将 MCP 工具定义转换为 LLM ToolDefinition 格式
// MCP 工具名使用 "mcp_{server}__{tool}" 格式，避免与本地 Skills 冲突
func MCPToolsToLLMTools(serverName string, mcpTools []MCPTool) []llm.ToolDefinition {
	tools := make([]llm.ToolDefinition, 0, len(mcpTools))

	for _, mt := range mcpTools {
		qualifiedName := fmt.Sprintf("%s%s__%s", MCPToolPrefix, serverName, mt.Name)

		var inputSchema interface{}
		if mt.InputSchema != nil {
			json.Unmarshal(mt.InputSchema, &inputSchema)
		}

		tools = append(tools, llm.ToolDefinition{
			Name:        qualifiedName,
			Description: mt.Description,
			InputSchema: inputSchema,
		})
	}

	return tools
}

// IsMCPTool 判断工具名是否为 MCP 工具
func IsMCPTool(toolName string) bool {
	return strings.HasPrefix(toolName, MCPToolPrefix)
}

// ParseMCPToolName 解析 MCP 工具名，返回 serverName 和 toolName
// 输入格式: "mcp_{server}__{tool}"
func ParseMCPToolName(qualifiedName string) (serverName, toolName string, ok bool) {
	if !IsMCPTool(qualifiedName) {
		return "", "", false
	}

	// 去掉 "mcp_" 前缀
	rest := qualifiedName[len(MCPToolPrefix):]

	// 找 "__" 分隔符
	for i := 0; i < len(rest)-1; i++ {
		if rest[i] == '_' && rest[i+1] == '_' {
			return rest[:i], rest[i+2:], true
		}
	}

	return "", "", false
}

// ExecuteMCPTool 通过 MCPManager 执行 MCP 工具，返回 LLM ToolResult
func ExecuteMCPTool(ctx context.Context, mgr *Manager, qualifiedName string, args map[string]interface{}) llm.ToolResult {
	serverName, toolName, ok := ParseMCPToolName(qualifiedName)
	if !ok {
		return llm.ToolResult{
			Content: fmt.Sprintf("Error: invalid MCP tool name %q", qualifiedName),
			IsError: true,
		}
	}

	instance := mgr.GetServer(serverName)
	if instance == nil {
		return llm.ToolResult{
			Content: fmt.Sprintf("Error: MCP server %q not found", serverName),
			IsError: true,
		}
	}

	result, err := instance.CallTool(ctx, toolName, args)
	if err != nil {
		return llm.ToolResult{
			Content: fmt.Sprintf("Error calling MCP tool %q on server %q: %v", toolName, serverName, err),
			IsError: true,
		}
	}

	if result.IsError {
		content := formatMCPContent(result.Content)
		return llm.ToolResult{
			Content: content,
			IsError: true,
		}
	}

	return llm.ToolResult{
		Content: formatMCPContent(result.Content),
	}
}

// formatMCPContent 将 MCP content blocks 格式化为文本
func formatMCPContent(contents []MCPContent) string {
	var parts []string
	for _, c := range contents {
		switch c.Type {
		case "text":
			parts = append(parts, c.Text)
		case "image":
			parts = append(parts, fmt.Sprintf("[image: %s]", c.MimeType))
		case "resource":
			parts = append(parts, fmt.Sprintf("[resource: %s]", c.URI))
		default:
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// GetAllLLMTools 从 MCPManager 获取所有 MCP 工具并转换为 LLM ToolDefinition
func GetAllLLMTools(mgr *Manager) []llm.ToolDefinition {
	if mgr == nil {
		return nil
	}

	var allTools []llm.ToolDefinition
	for _, serverName := range mgr.ServerNames() {
		instance := mgr.GetServer(serverName)
		if instance == nil || instance.Status() != StatusReady {
			continue
		}
		tools := MCPToolsToLLMTools(serverName, instance.Tools())
		allTools = append(allTools, tools...)
	}
	return allTools
}
