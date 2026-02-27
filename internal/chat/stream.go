package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// StreamChunk 流式响应块
type StreamChunk struct {
	Content   string `json:"content"`    // 内容片段
	Done      bool   `json:"done"`       // 是否完成
	Error     string `json:"error"`      // 错误信息
	TokenCount int   `json:"token_count"` // Token 数量
}

// StreamHandler 流式处理器
type StreamHandler struct {
	buffer      strings.Builder
	totalTokens int
	mu          sync.Mutex
	chunkChan   chan StreamChunk
	doneChan    chan struct{}
}

// NewStreamHandler 创建流式处理器
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{
		chunkChan: make(chan StreamChunk, 100),
		doneChan:  make(chan struct{}),
	}
}

// Chunks 返回块通道
func (h *StreamHandler) Chunks() <-chan StreamChunk {
	return h.chunkChan
}

// Done 返回完成通道
func (h *StreamHandler) Done() <-chan struct{} {
	return h.doneChan
}

// AddChunk 添加内容块
func (h *StreamHandler) AddChunk(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.buffer.WriteString(content)
	h.chunkChan <- StreamChunk{
		Content: content,
		Done:     false,
	}
}

// Complete 完成流式传输
func (h *StreamHandler) Complete(tokenCount int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.totalTokens = tokenCount
	h.chunkChan <- StreamChunk{
		Content:    "",
		Done:       true,
		TokenCount: tokenCount,
	}
	close(h.chunkChan)
	close(h.doneChan)
}

// Error 发生错误
func (h *StreamHandler) Error(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.chunkChan <- StreamChunk{
		Error: err.Error(),
		Done:  true,
	}
	close(h.chunkChan)
	close(h.doneChan)
}

// GetContent 获取完整内容
func (h *StreamHandler) GetContent() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.buffer.String()
}

// GetTotalTokens 获取总 Token 数
func (h *StreamHandler) GetTotalTokens() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.totalTokens
}

// Reset 重置处理器
func (h *StreamHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.buffer.Reset()
	h.totalTokens = 0
}

// SSEParser SSE 事件流解析器
type SSEParser struct {
	scanner *bufio.Scanner
}

// NewSSEParser 创建 SSE 解析器
func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{
		scanner: bufio.NewScanner(r),
	}
}

// Parse 解析 SSE 流
func (p *SSEParser) Parse() (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		for p.scanner.Scan() {
			line := p.scanner.Text()

			// 跳过空行
			if line == "" {
				continue
			}

			// 检查是否为 SSE 事件行
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			// 提取数据部分
			data := strings.TrimSpace(line[5:])

			// 检查是否为结束标记
			if data == "[DONE]" {
				chunkChan <- StreamChunk{Done: true}
				return
			}

			// 解析 JSON
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				errChan <- fmt.Errorf("failed to parse SSE data: %w", err)
				return
			}

			// 提取内容
			content := ""
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if c, ok := delta["content"].(string); ok {
							content = c
						}
					}
				}
			}

			// 提取使用信息
			var tokenCount int
			if usage, ok := chunk["usage"].(map[string]interface{}); ok {
				if tc, ok := usage["total_tokens"].(float64); ok {
					tokenCount = int(tc)
				}
			}

			chunkChan <- StreamChunk{
				Content:    content,
				TokenCount: tokenCount,
				Done:       false,
			}
		}

		if err := p.scanner.Err(); err != nil {
			errChan <- fmt.Errorf("scan error: %w", err)
		}
	}()

	return chunkChan, errChan
}

// StreamBuilder 流式内容构建器
type StreamBuilder struct {
	parts  []string
	total  int
	buffer bytes.Buffer
}

// NewStreamBuilder 创建流式构建器
func NewStreamBuilder() *StreamBuilder {
	return &StreamBuilder{
		parts: make([]string, 0, 100),
	}
}

// Append 追加内容
func (b *StreamBuilder) Append(content string) {
	b.parts = append(b.parts, content)
	b.total += len(content)
	b.buffer.WriteString(content)
}

// GetLatest 获取最新追加的内容
func (b *StreamBuilder) GetLatest() string {
	if len(b.parts) == 0 {
		return ""
	}
	return b.parts[len(b.parts)-1]
}

// GetAll 获取所有内容
func (b *StreamBuilder) GetAll() string {
	return b.buffer.String()
}

// GetParts 获取所有部分
func (b *StreamBuilder) GetParts() []string {
	return b.parts
}

// GetTotalLength 获取总长度
func (b *StreamBuilder) GetTotalLength() int {
	return b.total
}

// Reset 重置构建器
func (b *StreamBuilder) Reset() {
	b.parts = b.parts[:0]
	b.total = 0
	b.buffer.Reset()
}

// HasContent 检查是否有内容
func (b *StreamBuilder) HasContent() bool {
	return b.total > 0
}

// Merge 合并部分内容（用于减少碎片）
func (b *StreamBuilder) Merge(threshold int) {
	if len(b.parts) <= threshold {
		return
	}

	// 合并前 threshold 个部分
	merged := strings.Join(b.parts[:threshold], "")
	b.parts = append([]string{merged}, b.parts[threshold:]...)
}
