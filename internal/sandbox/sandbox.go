package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// Config 沙箱配置（由 main 从 config.SandboxConfig 填充）
type Config struct {
	WorkDir        string   // 工作目录，为空时使用默认 ~/.edge-proxy/sandbox
	TimeoutSeconds int      // 超时秒数，<=0 时用 30
	BashCommand    string   // bash 可执行路径，为空时 "bash"
	ExtraBlacklist []string // 额外黑名单子串
}

// Executor 沙箱执行器接口
type Executor interface {
	Run(ctx context.Context, script string, workDir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
	IsBlocked(script string) (blocked bool, reason string)
}

// defaultBlacklist 内置危险命令/模式黑名单（子串匹配，大小写不敏感）
var defaultBlacklist = []string{
	"rm -rf /",
	"rm -rf / ",
	"rm -rf /*",
	"mkfs.",
	"dd if=",
	">/dev/sd",
	"> /dev/sd",
	"chmod 777",
	":(){ :|:& };:", // fork bomb
	"| bash",
	"| sh ",
	"| sh\n",
	"bash <(",
	"sh <(",
	"eval ",
	"$(curl",
	"$(wget",
	">/dev/hda",
	"> /dev/hda",
	"mkswap",
	"swapoff",
	"sysctl",
	"/etc/passwd",
	"/etc/shadow",
	"sudo ",
	" su ",
	"chown ",
	"useradd",
	"userdel",
	"passwd ",
	"nohup",
	"disown",
	"format ",
	"fdisk ",
	"parted ",
}

type executor struct {
	workDir     string
	timeoutSec  int
	bashCommand string
	blacklist   []string
}

// New 根据配置创建沙箱执行器；workDir 为空时使用默认目录并尝试创建
func New(cfg Config) (Executor, error) {
	workDir := cfg.WorkDir
	if workDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		workDir = filepath.Join(home, ".edge-proxy", "sandbox")
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		abs = workDir
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("sandbox work_dir: %w", err)
	}

	timeoutSec := cfg.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	bashCmd := strings.TrimSpace(cfg.BashCommand)
	if bashCmd == "" {
		bashCmd = "bash"
	}

	blacklist := make([]string, 0, len(defaultBlacklist)+len(cfg.ExtraBlacklist))
	blacklist = append(blacklist, defaultBlacklist...)
	for _, s := range cfg.ExtraBlacklist {
		if t := strings.TrimSpace(s); t != "" {
			blacklist = append(blacklist, t)
		}
	}

	return &executor{
		workDir:     abs,
		timeoutSec:  timeoutSec,
		bashCommand: bashCmd,
		blacklist:   blacklist,
	}, nil
}

// IsBlocked 检查脚本是否命中黑名单
func (e *executor) IsBlocked(script string) (blocked bool, reason string) {
	lower := strings.ToLower(strings.TrimSpace(script))
	for _, pattern := range e.blacklist {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true, "blocked by blacklist: " + pattern
		}
	}
	return false, ""
}

// Run 在沙箱中执行脚本；workDir 为空时使用配置的默认工作目录；timeout <= 0 时使用配置默认超时
func (e *executor) Run(ctx context.Context, script string, workDir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if blocked, reason := e.IsBlocked(script); blocked {
		return "", "", -1, fmt.Errorf("script %s", reason)
	}

	dir := workDir
	if dir == "" {
		dir = e.workDir
	} else {
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		// 禁止逃逸到工作目录之外（必须落在 e.workDir 下）
		if abs != e.workDir && !strings.HasPrefix(abs, e.workDir+string(filepath.Separator)) {
			return "", "", -1, fmt.Errorf("work_dir must be inside sandbox: %s", e.workDir)
		}
		dir = abs
	}

	if timeout <= 0 {
		timeout = time.Duration(e.timeoutSec) * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, e.bashCommand, "-c", script)
	cmd.Dir = dir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	logger.Debug("Sandbox run: work_dir=%s timeout=%v", cmd.Dir, timeout)
	if runErr := cmd.Run(); runErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return outBuf.String(), errBuf.String(), -1, fmt.Errorf("timeout after %v", timeout)
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode(), runErr
		}
		return outBuf.String(), errBuf.String(), -1, runErr
	}

	return outBuf.String(), errBuf.String(), 0, nil
}
