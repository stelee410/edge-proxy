package logger

import (
	"sync"
	"time"
)

// LogEntry 日志条目
type LogEntry struct {
	Level     Level
	Timestamp time.Time
	Message   string
}

// LogBuffer 日志缓冲器
type LogBuffer struct {
	entries []LogEntry
	maxSize int
	mu      sync.RWMutex
	logChan chan LogEntry
}

var (
	buffer     *LogBuffer
	bufferOnce sync.Once
)

// InitBuffer 初始化日志缓冲器
func InitBuffer(maxSize int, logChan chan LogEntry) {
	bufferOnce.Do(func() {
		buffer = &LogBuffer{
			entries: make([]LogEntry, 0, maxSize),
			maxSize: maxSize,
			logChan: logChan,
		}
	})
}

// AddEntry 添加日志条目（循环缓冲 + 非阻塞发送）
func (b *LogBuffer) AddEntry(level Level, message string) {
	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
	}

	b.mu.Lock()
	// 循环缓冲：如果超过最大大小，删除最旧的条目
	if len(b.entries) >= b.maxSize {
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)
	b.mu.Unlock()

	// 非阻塞发送到通道
	if b.logChan != nil {
		select {
		case b.logChan <- entry:
			// 发送成功
		default:
			// 通道满时丢弃（防止阻塞 Proxy）
		}
	}
}

// GetRecent 获取最近 N 条日志
func (b *LogBuffer) GetRecent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	start := len(b.entries) - n
	if start < 0 {
		start = 0
	}
	result := make([]LogEntry, len(b.entries)-start)
	copy(result, b.entries[start:])
	return result
}
