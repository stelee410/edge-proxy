package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"linkyun-edge-proxy/internal/logger"
)

// StdioTransport 通过子进程 stdin/stdout 通信的 MCP 传输
type StdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	closed  bool
}

// StdioConfig stdio 传输配置
type StdioConfig struct {
	Command string            // 可执行文件路径
	Args    []string          // 命令行参数
	Env     map[string]string // 环境变量
	WorkDir string            // 工作目录
}

// NewStdioTransport 创建 stdio 传输
func NewStdioTransport(ctx context.Context, cfg StdioConfig) (*StdioTransport, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	// 设置环境变量
	if len(cfg.Env) > 0 {
		env := cmd.Environ()
		for k, v := range cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// stderr 用于日志
	cmd.Stderr = &stderrLogger{}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP server process: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	logger.Debug("MCP stdio transport started: %s %v (pid=%d)", cfg.Command, cfg.Args, cmd.Process.Pid)

	return &StdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: scanner,
	}, nil
}

// Send 发送消息到子进程 stdin
func (t *StdioTransport) Send(msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	// 每条消息以换行符分隔
	if _, err := t.stdin.Write(msg); err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}
	if _, err := t.stdin.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// Receive 从子进程 stdout 接收消息
func (t *StdioTransport) Receive() ([]byte, error) {
	if t.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		return nil, io.EOF
	}

	return t.scanner.Bytes(), nil
}

// Close 关闭传输，终止子进程
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	t.stdin.Close()

	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	t.cmd.Wait()

	logger.Debug("MCP stdio transport closed")
	return nil
}

// stderrLogger 将 MCP server 的 stderr 输出到日志
type stderrLogger struct{}

func (l *stderrLogger) Write(p []byte) (n int, err error) {
	logger.Debug("MCP server stderr: %s", string(p))
	return len(p), nil
}
