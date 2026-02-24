package logger

import (
	"fmt"
	"log"
	"strings"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var (
	currentLevel = INFO
	enableStdout = true
)

// SetLevel 设置日志级别
func SetLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		currentLevel = DEBUG
	case "info":
		currentLevel = INFO
	case "warn", "warning":
		currentLevel = WARN
	case "error":
		currentLevel = ERROR
	default:
		currentLevel = INFO
	}
}

// Debug 输出 DEBUG 级别日志
func Debug(format string, args ...interface{}) {
	if currentLevel <= DEBUG {
		msg := fmt.Sprintf(format, args...)
		if enableStdout {
			log.Printf("[DEBUG] " + msg)
		}

		// 添加到缓冲器
		if buffer != nil {
			buffer.AddEntry(DEBUG, msg)
		}
	}
}

// Info 输出 INFO 级别日志
func Info(format string, args ...interface{}) {
	if currentLevel <= INFO {
		msg := fmt.Sprintf(format, args...)
		if enableStdout {
			log.Printf("[INFO]  " + msg)
		}

		// 添加到缓冲器
		if buffer != nil {
			buffer.AddEntry(INFO, msg)
		}
	}
}

// Warn 输出 WARN 级别日志
func Warn(format string, args ...interface{}) {
	if currentLevel <= WARN {
		msg := fmt.Sprintf(format, args...)
		if enableStdout {
			log.Printf("[WARN]  " + msg)
		}

		// 添加到缓冲器
		if buffer != nil {
			buffer.AddEntry(WARN, msg)
		}
	}
}

// Error 输出 ERROR 级别日志
func Error(format string, args ...interface{}) {
	if currentLevel <= ERROR {
		msg := fmt.Sprintf(format, args...)
		if enableStdout {
			log.Printf("[ERROR] " + msg)
		}

		// 添加到缓冲器
		if buffer != nil {
			buffer.AddEntry(ERROR, msg)
		}
	}
}

// MaskToken 脱敏 Token 显示
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:6] + "..." + token[len(token)-4:]
}

// DisableStdout 禁用标准输出（TUI 退出后调用）
func DisableStdout() {
	enableStdout = false
}
