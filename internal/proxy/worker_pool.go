package proxy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// WorkerPool 固定大小的 Worker 池，从 OrderCache 消费订单并处理。
type WorkerPool struct {
	maxWorkers int
	cache      *OrderCache
	proxy      *Proxy
	active     atomic.Int32 // 当前正在处理的 worker 数
	wg         sync.WaitGroup
}

// NewWorkerPool 创建 Worker 池。
func NewWorkerPool(maxWorkers int, cache *OrderCache, p *Proxy) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 2
	}
	return &WorkerPool{
		maxWorkers: maxWorkers,
		cache:      cache,
		proxy:      p,
	}
}

// ActiveCount 返回正在处理中的 worker 数量。
func (wp *WorkerPool) ActiveCount() int {
	return int(wp.active.Load())
}

// Start 启动 maxWorkers 个 goroutine，监听 OrderCache.WorkCh()。
func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.maxWorkers; i++ {
		go wp.workerLoop(ctx, i)
	}
	logger.Info("[WorkerPool] started %d worker(s)", wp.maxWorkers)
}

func (wp *WorkerPool) workerLoop(ctx context.Context, id int) {
	logger.Info("[Worker-%d] started", id)
	for {
		select {
		case <-ctx.Done():
			logger.Info("[Worker-%d] stopped", id)
			return
		case order, ok := <-wp.cache.WorkCh():
			if !ok {
				return
			}
			wp.processOrder(ctx, order)
		}
	}
}

// processOrder 处理单个订单：通知开始 → 执行 LLM → 通知结果。
func (wp *WorkerPool) processOrder(ctx context.Context, order *Order) {
	wp.active.Add(1)
	wp.wg.Add(1)
	defer func() {
		wp.active.Add(-1)
		wp.wg.Done()
	}()

	reqID := order.RequestID
	logger.Info("[WorkerPool] start processing order: request_id=%s session=%s", reqID, order.SessionUUID)
	wp.cache.MarkProcessing(reqID)

	start := time.Now()

	// 始终使用非流式处理（Quick-ACK 已经关闭了原始 SSE 流）
	// 结果通过 notifyUserSaveDB 持久化后推送给用户
	req := *order.Request
	req.Stream = false

	var finalContent string
	var finalErr string

	// capturedNotifyContent 用于捕获工具结果中的 [NOTIFY] 内容。
	// 当工具（如 suno_generate）以 [NOTIFY] 前缀返回内容时，handleToolNotify 会
	// 通过 SSE 发送即时通知，并通过此 context 回调让 Worker 知道需要将该内容存 DB。
	var capturedNotifyContent string
	captureCtx := context.WithValue(ctx, notifyCaptureFn{}, func(msg string) {
		capturedNotifyContent = msg
	})

	sink := func(_ context.Context, resp *EdgeResponse) {
		if resp.Success {
			finalContent = resp.Content
		} else {
			finalErr = resp.Error
		}
	}
	wp.proxy.handleRequest(captureCtx, &req, sink)

	elapsed := time.Since(start)

	if finalErr != "" {
		errMsg := fmt.Sprintf("❌ 处理您的消息时遇到了问题：%s", finalErr)
		wp.proxy.notifyUserSaveDB(ctx, order.Request, errMsg, nil)
		wp.cache.MarkFailed(reqID)
		logger.Error("[WorkerPool] order failed: request_id=%s elapsed=%s err=%s", reqID, elapsed, finalErr)
		return
	}

	// 优先使用 [NOTIFY] 捕获内容（工具直接推送场景，如 suno_generate 返回歌曲链接）
	if finalContent == "" && capturedNotifyContent != "" {
		logger.Info("[WorkerPool] order done via [NOTIFY]: request_id=%s elapsed=%s", reqID, elapsed)
		// SSE 已在 handleToolNotify 中推送，这里只需持久化到 DB
		wp.proxy.notifyUserSaveDB(ctx, order.Request, capturedNotifyContent, nil)
		wp.cache.MarkDone(reqID)
		return
	}

	// 普通 LLM 响应，持久化并推送
	if finalContent == "" {
		finalContent = "（处理完成，但模型未返回文字内容）"
	}
	wp.proxy.notifyUserSaveDB(ctx, order.Request, finalContent, nil)
	wp.cache.MarkDone(reqID)
	logger.Info("[WorkerPool] order done: request_id=%s elapsed=%s", reqID, elapsed)
}

// WaitIdle 等待所有 active worker 完成，最多等待 timeout。
func (wp *WorkerPool) WaitIdle(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.Info("[WorkerPool] all workers idle")
	case <-time.After(timeout):
		logger.Warn("[WorkerPool] wait idle timeout after %s (%d still active)", timeout, wp.active.Load())
	}
}
