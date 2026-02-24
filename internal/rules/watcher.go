package rules

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"linkyun-edge-proxy/internal/logger"
)

const (
	debounceDelay = 500 * time.Millisecond
)

// Watcher 文件系统监听器，监控 rules 目录的变化并触发重载
type Watcher struct {
	engine      *Engine
	directories []string
	watcher     *fsnotify.Watcher
}

// NewWatcher 创建文件监听器
func NewWatcher(engine *Engine, directories []string) *Watcher {
	return &Watcher{
		engine:      engine,
		directories: directories,
	}
}

// Start 启动文件监听，在 context 取消时自动停止
func (w *Watcher) Start(ctx context.Context) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = fsWatcher

	// 添加监听目录（递归）
	for _, dir := range w.directories {
		if err := w.addDirRecursive(dir); err != nil {
			logger.Warn("Failed to watch directory %q: %v", dir, err)
		}
	}

	go w.watchLoop(ctx)
	return nil
}

// addDirRecursive 递归添加目录到 watcher
func (w *Watcher) addDirRecursive(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在，静默跳过
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				logger.Warn("Failed to watch %q: %v", path, err)
			}
		}
		return nil
	})
}

// watchLoop 主监听循环，带 debounce 防抖
func (w *Watcher) watchLoop(ctx context.Context) {
	defer w.watcher.Close()

	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// 只关注 .mdc 文件的变化
			if !isMDCFile(event.Name) && !isDirectory(event.Name) {
				continue
			}

			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) ||
				event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {

				logger.Debug("Rules file change detected: %s (%s)", event.Name, event.Op)

				// 如果是新创建的目录，加入监听
				if event.Has(fsnotify.Create) && isDirectory(event.Name) {
					w.watcher.Add(event.Name)
				}

				// 防抖：500ms 内的多次变化只触发一次重载
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDelay, func() {
					w.reload()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			logger.Warn("Rules watcher error: %v", err)
		}
	}
}

// reload 重新加载规则
func (w *Watcher) reload() {
	logger.Info("Rules file change detected, reloading...")
	if err := w.engine.LoadRules(); err != nil {
		logger.Error("Failed to reload rules: %v", err)
		return
	}
	logger.Info("Rules reloaded successfully: %d rules loaded", w.engine.RuleCount())
}

// isMDCFile 判断文件是否为 .mdc 文件
func isMDCFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".mdc"
}

// isDirectory 判断路径是否为目录
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
