package mcp

import "encoding/json"

// MCP 协议版本
const (
	ProtocolVersion = "2024-11-05"
	ClientName      = "linkyun-edge-proxy"
	ClientVersion   = "1.0.0"
)

// MCPTool MCP 服务器提供的工具定义
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPToolResult MCP 工具调用结果
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent MCP 内容块
type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// MCPResource MCP 服务器提供的资源
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPResourceContent 资源内容
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// InitializeParams initialize 请求参数
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    Capabilities   `json:"capabilities"`
	ClientInfo      Implementation `json:"clientInfo"`
}

// InitializeResult initialize 响应结果
type InitializeResult struct {
	ProtocolVersion string               `json:"protocolVersion"`
	Capabilities    ServerCapabilities   `json:"capabilities"`
	ServerInfo      Implementation       `json:"serverInfo"`
}

// Capabilities 客户端能力
type Capabilities struct {
	// 客户端暂不声明特殊能力
}

// ServerCapabilities 服务器能力
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability 工具能力
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability 资源能力
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability 提示能力
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Implementation 实现信息
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResult tools/list 响应
type ToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// ToolCallParams tools/call 请求参数
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ResourcesListResult resources/list 响应
type ResourcesListResult struct {
	Resources []MCPResource `json:"resources"`
}

// ResourceReadParams resources/read 请求参数
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceReadResult resources/read 响应
type ResourceReadResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// Transport MCP 传输层接口
type Transport interface {
	// Send 发送 JSON-RPC 消息
	Send(msg []byte) error
	// Receive 接收 JSON-RPC 消息
	Receive() ([]byte, error)
	// Close 关闭传输
	Close() error
}
