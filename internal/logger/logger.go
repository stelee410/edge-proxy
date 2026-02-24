package logger

import (
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

var currentLevel = INFO

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
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Info 输出 INFO 级别日志
func Info(format string, args ...interface{}) {
	if currentLevel <= INFO {
		log.Printf("[INFO]  "+format, args...)
	}
}

// Warn 输出 WARN 级别日志
func Warn(format string, args ...interface{}) {
	if currentLevel <= WARN {
		log.Printf("[WARN]  "+format, args...)
	}
}

// Error 输出 ERROR 级别日志
func Error(format string, args ...interface{}) {
	if currentLevel <= ERROR {
		log.Printf("[ERROR] "+format, args...)
	}
}

// MaskToken 脱敏 Token 显示
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:6] + "..." + token[len(token)-4:]
}
