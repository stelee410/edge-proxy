package mcp

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// JSON-RPC 2.0 错误码
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Request JSON-RPC 2.0 请求
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response JSON-RPC 2.0 响应
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Notification JSON-RPC 2.0 通知（无 ID）
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// RPCError JSON-RPC 2.0 错误
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// IDGenerator 请求 ID 生成器
type IDGenerator struct {
	counter atomic.Int64
}

// Next 生成下一个请求 ID
func (g *IDGenerator) Next() int64 {
	return g.counter.Add(1)
}

// NewRequest 创建 JSON-RPC 2.0 请求
func NewRequest(id int64, method string, params interface{}) *Request {
	return &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewNotification 创建 JSON-RPC 2.0 通知
func NewNotification(method string, params interface{}) *Notification {
	return &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

// EncodeRequest 编码请求为 JSON
func EncodeRequest(req *Request) ([]byte, error) {
	return json.Marshal(req)
}

// EncodeNotification 编码通知为 JSON
func EncodeNotification(n *Notification) ([]byte, error) {
	return json.Marshal(n)
}

// DecodeResponse 解码 JSON-RPC 响应
func DecodeResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}

// DecodeResult 解码响应中的 result 字段
func DecodeResult[T any](resp *Response) (*T, error) {
	if resp.Error != nil {
		return nil, resp.Error
	}
	var result T
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to decode result: %w", err)
	}
	return &result, nil
}
