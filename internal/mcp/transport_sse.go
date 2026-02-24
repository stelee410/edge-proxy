package mcp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// SSETransport 通过 HTTP SSE 通信的 MCP 传输
type SSETransport struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
	messages   chan []byte
	postURL    string // SSE 服务返回的消息发送端点
	mu         sync.Mutex
	closed     bool
	closeFunc  func()
}

// SSEConfig SSE 传输配置
type SSEConfig struct {
	URL     string            // SSE 服务器 URL
	Headers map[string]string // 自定义请求头
}

// NewSSETransport 创建 SSE 传输
func NewSSETransport(cfg SSEConfig) (*SSETransport, error) {
	t := &SSETransport{
		baseURL: strings.TrimSuffix(cfg.URL, "/"),
		httpClient: &http.Client{
			Timeout: 0, // SSE 连接不超时
		},
		headers:  cfg.Headers,
		messages: make(chan []byte, 100),
	}

	if err := t.connect(); err != nil {
		return nil, err
	}

	return t, nil
}

// connect 建立 SSE 连接
func (t *SSETransport) connect() error {
	sseURL := t.baseURL + "/sse"
	req, err := http.NewRequest("GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
	}

	// 启动 SSE 读取协程
	go t.readSSE(resp.Body)

	// 等待接收 endpoint 事件
	select {
	case msg := <-t.messages:
		// 第一条消息应该是 endpoint 信息
		t.postURL = t.baseURL + string(msg)
		logger.Debug("MCP SSE transport connected, post URL: %s", t.postURL)
	case <-time.After(10 * time.Second):
		resp.Body.Close()
		return fmt.Errorf("timeout waiting for SSE endpoint event")
	}

	return nil
}

// readSSE 读取 SSE 事件流
func (t *SSETransport) readSSE(body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var eventType string

	for scanner.Scan() {
		if t.closed {
			return
		}

		line := scanner.Text()

		if line == "" {
			// 空行表示事件结束
			eventType = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			switch eventType {
			case "endpoint":
				// endpoint 事件，发送到消息通道
				select {
				case t.messages <- []byte(data):
				default:
				}
			case "message":
				// 消息事件
				select {
				case t.messages <- []byte(data):
				default:
					logger.Warn("MCP SSE message channel full, dropping message")
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && !t.closed {
		logger.Warn("MCP SSE read error: %v", err)
	}
}

// Send 通过 HTTP POST 发送消息
func (t *SSETransport) Send(msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	if t.postURL == "" {
		return fmt.Errorf("SSE endpoint not available")
	}

	req, err := http.NewRequest("POST", t.postURL, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receive 从 SSE 消息通道接收消息
func (t *SSETransport) Receive() ([]byte, error) {
	if t.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	msg, ok := <-t.messages
	if !ok {
		return nil, io.EOF
	}

	return msg, nil
}

// Close 关闭 SSE 传输
func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	if t.closeFunc != nil {
		t.closeFunc()
	}
	close(t.messages)

	logger.Debug("MCP SSE transport closed")
	return nil
}
