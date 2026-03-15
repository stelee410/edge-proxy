package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"linkyun-edge-proxy/internal/config"
	"linkyun-edge-proxy/internal/docparse"
	"linkyun-edge-proxy/internal/llm"
	"linkyun-edge-proxy/internal/logger"
	"linkyun-edge-proxy/internal/mcp"
	"linkyun-edge-proxy/internal/rules"
	"linkyun-edge-proxy/internal/sandbox"
	"linkyun-edge-proxy/internal/skills"
)

// isTimeoutError 判断错误是否为超时类错误（网络超时或 context deadline）
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// EdgeAttachment 附件格式（与服务器 models.Attachment 对应）
type EdgeAttachment struct {
	Type     string `json:"type"` // image, audio, file
	Token    string `json:"token,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Name     string `json:"name,omitempty"`
}

// EdgeRequest 服务器发来的请求（与服务端 models.EdgeRequest 对应）
type EdgeRequest struct {
	RequestID    string          `json:"request_id"`
	AgentUUID    string          `json:"agent_uuid"`
	SessionUUID  string          `json:"session_uuid"`
	Type         string          `json:"type"` // "chat" | "simulate"
	SystemPrompt string          `json:"system_prompt"`
	Messages     []EdgeMessage   `json:"messages"`
	Attachments  json.RawMessage `json:"attachments,omitempty"` // 当前用户消息的图片附件（image_upload）
	Model        string          `json:"model"`
	Temperature  float64         `json:"temperature"`
	MaxTokens    int             `json:"max_tokens"`
	MemoryEnabled bool           `json:"memory_enabled"` // 是否启用长期记忆（具体实现由 Edge 自行决定）
	LLMProvider  string          `json:"llm_provider"`   // Cloud Agent 指定的 LLM provider（优先使用）
	CreatorID    string          `json:"creator_id"`
	UserID       string          `json:"user_id,omitempty"` // 用户 ID，用于 memory 按用户隔离（跨会话）
	Stream       bool            `json:"stream,omitempty"`  // 是否使用流式响应
	Timestamp    time.Time       `json:"timestamp"`
}

// EdgeStreamChunk 流式响应数据块（NDJSON 格式发送到服务器）
type EdgeStreamChunk struct {
	RequestID string          `json:"request_id"`
	AgentUUID string          `json:"agent_uuid"`
	Type      string          `json:"type"`               // "delta" | "done" | "error"
	Content   string          `json:"content,omitempty"`  // delta 文本增量
	Model     string          `json:"model,omitempty"`    // done 时携带实际模型名
	Usage     *EdgeTokenUsage `json:"usage,omitempty"`    // done 时携带 token 用量
	Metadata  map[string]any  `json:"metadata,omitempty"` // done 时携带 metadata
	Error     string          `json:"error,omitempty"`    // error 时的错误信息
}

// EdgeMessage 消息格式
type EdgeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// EdgeResponse 返回给服务器的响应
type EdgeResponse struct {
	RequestID   string          `json:"request_id"`
	AgentUUID   string          `json:"agent_uuid"`
	Success     bool            `json:"success"`
	Content     string          `json:"content,omitempty"`
	Model       string          `json:"model,omitempty"`
	Usage       *EdgeTokenUsage `json:"usage,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"` // 供 client 使用，如 memory_actions
	Error       string          `json:"error,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
	AudioBase64 string          `json:"audio_base64,omitempty"` // TTS 音频数据（Base64 编码，fallback）
	AudioFormat string          `json:"audio_format,omitempty"` // 音频格式（mp3, wav 等）
	AudioURL    string          `json:"audio_url,omitempty"`    // 音频下载 URL（替代 base64）
	// Queued=true 表示请求已进入本地队列，服务端不应保存消息到 DB，
	// 最终结果将通过 /edge/notify save_to_db=true 推送。
	Queued bool `json:"queued,omitempty"`
}

// MemoryAction 本次对话中的记忆操作，供 client 展示
type MemoryAction struct {
	Action  string `json:"action"`  // "save" | "delete"
	Content string `json:"content,omitempty"`
	Keyword string `json:"keyword,omitempty"`
	Result  string `json:"result,omitempty"`
}

// EdgeTokenUsage Token 用量
type EdgeTokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Proxy Edge 代理核心逻辑
type Proxy struct {
	config          *config.Config
	llmRegistry     *llm.Registry
	defaultProvider llm.Provider         // 默认 Provider（可能是 FallbackProvider）
	rulesEngine     *rules.Engine        // Rules 引擎（可为 nil）
	skillPipeline   *skills.Pipeline     // Skill 管道（可为 nil）
	toolExecutor    *ToolExecutor        // Tool 执行器（可为 nil）
	resourceMgr     *mcp.ResourceManager // MCP 资源管理器（可为 nil）
	httpClient      *http.Client
	running         bool
	requestCount    int
	errorCount      int
	statsCollector  *StatsCollector  // 统计收集器
	statsChan       chan<- ProxyStats // 统计数据通道
	orderCache      *OrderCache      // 本地订单持久化队列
	workerPool      *WorkerPool      // 并发 Worker 池
}

// New 创建 Proxy 实例
func New(cfg *config.Config, registry *llm.Registry) *Proxy {
	// 构建带 fallback 的默认 Provider
	defaultProvider, err := registry.BuildFallbackProvider(cfg.LLM.Fallback)
	if err != nil {
		// fallback 构建失败时回退到普通默认 Provider
		defaultProvider, _ = registry.Default()
	}

	cc := cfg.Concurrency
	if cc.MaxWorkers <= 0 {
		cc.MaxWorkers = 2
	}
	if cc.MaxQueueSize <= 0 {
		cc.MaxQueueSize = 10
	}
	if cc.DBPath == "" {
		cc.DBPath = "./orders.db"
	}

	orderCache, err := NewOrderCache(cc.DBPath, cc.MaxQueueSize)
	if err != nil {
		logger.Warn("[Proxy] failed to open order cache (%s), falling back to no persistence: %v", cc.DBPath, err)
		orderCache = nil
	}

	p := &Proxy{
		config:          cfg,
		llmRegistry:     registry,
		defaultProvider: defaultProvider,
		httpClient: &http.Client{
			Timeout: cfg.PollTimeout + 10*time.Second,
		},
		statsCollector: NewStatsCollector(),
		orderCache:     orderCache,
	}

	if orderCache != nil {
		p.workerPool = NewWorkerPool(cc.MaxWorkers, orderCache, p)
	}

	return p
}

// SetRulesEngine 设置 Rules 引擎
func (p *Proxy) SetRulesEngine(engine *rules.Engine) {
	p.rulesEngine = engine
}

// GetRulesEngine 获取 Rules 引擎
func (p *Proxy) GetRulesEngine() *rules.Engine {
	return p.rulesEngine
}

// SetSkillPipeline 设置 Skill 管道和 Tool 执行器
func (p *Proxy) SetSkillPipeline(pipeline *skills.Pipeline) {
	p.skillPipeline = pipeline
	if pipeline != nil {
		p.toolExecutor = NewToolExecutor(pipeline.GetRegistry())
		p.toolExecutor.SetMemoryAPI(p)
	}
}

// GetSkillPipeline 获取 Skill 管道
func (p *Proxy) GetSkillPipeline() *skills.Pipeline {
	return p.skillPipeline
}

// SetMCPManager 设置 MCP 管理器到 ToolExecutor
func (p *Proxy) SetMCPManager(mgr *mcp.Manager) {
	if p.toolExecutor != nil {
		p.toolExecutor.SetMCPManager(mgr)
	}
}

// SetSandbox 设置 Bash 沙箱到 ToolExecutor（用于 run_shell 工具）
func (p *Proxy) SetSandbox(sb sandbox.Executor) {
	if p.toolExecutor != nil && sb != nil {
		p.toolExecutor.SetSandbox(sb)
	}
}

// SetResourceManager 设置 MCP 资源管理器
func (p *Proxy) SetResourceManager(rm *mcp.ResourceManager) {
	p.resourceMgr = rm
}

// EnsureToolExecutor 确保 ToolExecutor 存在（当 Skills 未启用时，用于 MCP 等工具）
func (p *Proxy) EnsureToolExecutor() {
	if p.toolExecutor == nil {
		p.toolExecutor = NewToolExecutor(nil)
		p.toolExecutor.SetMemoryAPI(p)
	}
}

// SetStatsChannel 设置统计数据通道
func (p *Proxy) SetStatsChannel(ch chan<- ProxyStats) {
	p.statsChan = ch
}

// publishStats 发布统计数据（非阻塞）
func (p *Proxy) publishStats() {
	if p.statsChan != nil {
		stats := p.statsCollector.GetStats()
		select {
		case p.statsChan <- stats:
			// 发送成功
		default:
			// 通道满时丢弃
		}
	}
}


// Run 启动代理主循环
func (p *Proxy) Run(ctx context.Context) error {
	logger.Info("Starting Edge Proxy for agent %s", p.config.AgentUUID)
	logger.Info("Server: %s", p.config.ServerURL)
	logger.Info("LLM Providers: %v, Default: %s", p.llmRegistry.ProviderNames(), p.llmRegistry.DefaultName())
	logger.Info("Edge Token: %s", logger.MaskToken(p.config.EdgeToken))
	logger.Info("Heartbeat interval: %s, Poll timeout: %s", p.config.HeartbeatInterval, p.config.PollTimeout)

	cc := p.config.Concurrency
	if p.orderCache != nil {
		logger.Info("Order cache: max_workers=%d, max_queue=%d, db=%s",
			cc.MaxWorkers, cc.MaxQueueSize, cc.DBPath)
	}

	// 连接服务器（带重试）
	if err := p.connectWithRetry(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	// 优雅退出：先通知用户，再断开连接
	defer p.gracefulShutdown(ctx)

	// 更新连接状态
	p.statsCollector.SetConnected(true)
	p.publishStats()

	p.running = true
	logger.Info("Connected successfully, entering main loop")

	// 启动 Worker 池
	if p.workerPool != nil {
		p.workerPool.Start(ctx)
		p.recoverPendingOrders(ctx)
	}

	go p.heartbeatLoop(ctx)

	// 定期发布统计信息
	statsTicker := time.NewTicker(2 * time.Second)
	defer statsTicker.Stop()

	retryDelay := 2 * time.Second
	maxRetryDelay := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down (processed %d requests, %d errors)", p.requestCount, p.errorCount)
			p.statsCollector.SetConnected(false)
			p.publishStats()
			p.running = false
			return nil
		case <-statsTicker.C:
			p.publishStats()
		default:
			if err := p.pollAndProcess(ctx); err != nil {
				logger.Warn("Poll error: %v (retry in %s)", err, retryDelay)
				p.errorCount++
				select {
				case <-ctx.Done():
					p.running = false
					return nil
				case <-time.After(retryDelay):
				}
				retryDelay = retryDelay * 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
			} else {
				retryDelay = 2 * time.Second
			}
		}
	}
}

// recoverPendingOrders 启动时从 SQLite 恢复上次未完成的订单。
// - processing 状态（崩溃中断）：通知用户重试，标记 failed。
// - queued 状态（排队中未执行）：重新派送给 Worker 池继续处理。
func (p *Proxy) recoverPendingOrders(ctx context.Context) {
	if p.orderCache == nil {
		return
	}
	orders, err := p.orderCache.LoadPendingOrders()
	if err != nil {
		logger.Warn("[Proxy] recover pending orders: %v", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	var recovered, aborted int
	for _, o := range orders {
		if o.Status == OrderStatusProcessing {
			// 上次崩溃时正在处理，无法继续，通知用户重试
			p.orderCache.MarkFailed(o.RequestID)
			p.notifyUser(o.Request, "status",
				"抱歉，我刚才意外重启，上次的请求没有完成，请重新发送。", nil)
			aborted++
		} else {
			// queued：重新分发给 Worker
			p.orderCache.Dispatch(o)
			recovered++
		}
	}
	logger.Info("[Proxy] recovered %d queued orders, aborted %d crashed orders", recovered, aborted)
}

// gracefulShutdown 优雅退出：通知所有排队中的用户，等待进行中任务完成，再断开连接。
func (p *Proxy) gracefulShutdown(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if p.orderCache != nil {
		// 通知所有 queued 订单用户
		queued, err := p.orderCache.AllQueuedOrders()
		if err == nil && len(queued) > 0 {
			logger.Info("[Proxy] notifying %d queued orders of shutdown", len(queued))
			for _, o := range queued {
				p.orderCache.MarkFailed(o.RequestID)
				p.notifyUser(o.Request, "status",
					"我已暂时下线，您的请求未能处理，请等我上线后重新发送。", nil)
			}
		}

		// 等待进行中的 Worker 完成（最多 25 秒）
		if p.workerPool != nil {
			p.workerPool.WaitIdle(25 * time.Second)
		}

		p.orderCache.Close()
	}

	p.disconnect(shutdownCtx)
}

// connectWithRetry 带重试的连接
func (p *Proxy) connectWithRetry(ctx context.Context) error {
	maxAttempts := 5
	delay := 2 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := p.connect(ctx)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			return fmt.Errorf("failed after %d attempts: %w", maxAttempts, err)
		}
		logger.Warn("Connect attempt %d/%d failed: %v (retry in %s)", attempt, maxAttempts, err, delay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = delay * 2
	}
	return fmt.Errorf("connect failed")
}

// connect 连接到 Linkyun Server
func (p *Proxy) connect(ctx context.Context) error {
	logger.Debug("Sending connect request...")
	body := map[string]string{"agent_uuid": p.config.AgentUUID}
	resp, err := p.doRequest(ctx, "POST", "/api/v1/edge/connect", body)
	if err != nil {
		return fmt.Errorf("connect request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid edge_token, please check your configuration")
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("agent_uuid does not match the edge_token, please verify your configuration")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connect failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var envelope struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&envelope)
	if envelope.Data != nil {
		logger.Info("Connected to server: agent=%v", envelope.Data["agent_name"])
	} else {
		logger.Info("Connected to server")
	}
	return nil
}

// disconnect 断开连接
func (p *Proxy) disconnect(ctx context.Context) {
	logger.Info("Disconnecting from server...")
	disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body := map[string]string{"agent_uuid": p.config.AgentUUID}
	resp, err := p.doRequest(disconnectCtx, "POST", "/api/v1/edge/disconnect", body)
	if err != nil {
		logger.Warn("Disconnect request failed: %v", err)
		return
	}
	resp.Body.Close()
	logger.Info("Disconnected from server")
}

// heartbeatLoop 心跳循环
func (p *Proxy) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(p.config.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.sendHeartbeat(ctx); err != nil {
				logger.Warn("Heartbeat failed: %v", err)
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (p *Proxy) sendHeartbeat(ctx context.Context) error {
	body := map[string]string{"agent_uuid": p.config.AgentUUID}
	resp, err := p.doRequest(ctx, "POST", "/api/v1/edge/heartbeat", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat returned status %d", resp.StatusCode)
	}
	logger.Debug("Heartbeat sent")
	return nil
}

// pollAndProcess 轮询服务端队列，将请求写入本地 OrderCache 或直接处理。
func (p *Proxy) pollAndProcess(ctx context.Context) error {
	url := fmt.Sprintf("/api/v1/edge/poll?agent_uuid=%s&timeout=%d",
		p.config.AgentUUID, int(p.config.PollTimeout.Seconds()))

	resp, err := p.doRequest(ctx, "GET", url, nil)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if isTimeoutError(err) || errors.Is(err, io.EOF) {
			logger.Debug("Poll: connection closed or timeout, reconnecting")
			return nil
		}
		return fmt.Errorf("poll request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("poll returned status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read poll response: %w", err)
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("failed to parse poll envelope: %w", err)
	}

	if !envelope.Success || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}

	var edgeReq EdgeRequest
	if err := json.Unmarshal(envelope.Data, &edgeReq); err != nil {
		return fmt.Errorf("failed to parse edge request: %w", err)
	}

	logger.Info("Received request: id=%s type=%s messages=%d stream=%v",
		edgeReq.RequestID, edgeReq.Type, len(edgeReq.Messages), edgeReq.Stream)
	p.requestCount++

	// 有本地 Worker 池：走本地队列模式
	if p.workerPool != nil {
		p.enqueueOrReject(ctx, &edgeReq)
		return nil
	}

	// 无本地 Worker 池（降级）：直接 goroutine 处理（原有行为）
	if edgeReq.Stream {
		go p.handleRequestStream(ctx, &edgeReq)
	} else {
		go p.handleRequest(ctx, &edgeReq, nil)
	}
	return nil
}

// enqueueOrReject 尝试将请求入本地队列；队列已满则立刻拒绝并通知用户。
func (p *Proxy) enqueueOrReject(ctx context.Context, edgeReq *EdgeRequest) {
	if p.orderCache.IsFull() {
		// 队列已满：拒绝，立刻给用户发 SSE 通知，同时向服务端发 Queued ACK
		rejectMsg := "抱歉，我现在任务较多，无法接受新的请求，请稍后再试。"
		logger.Warn("[Proxy] order queue full, rejecting request %s", edgeReq.RequestID)
		go p.notifyUser(edgeReq, "status", rejectMsg, map[string]any{"stage": "rejected"})
		go p.sendQuickACK(ctx, edgeReq) // 通知服务端停止等待（不存 DB）
		p.orderCache.MarkRejected(edgeReq.RequestID)
		return
	}

	order, err := p.orderCache.Add(edgeReq)
	if err != nil {
		logger.Error("[Proxy] failed to save order %s: %v", edgeReq.RequestID, err)
		go p.notifyUser(edgeReq, "status", "内部错误，请重试。", map[string]any{"stage": "error"})
		go p.sendQuickACK(ctx, edgeReq)
		return
	}

	// 向用户发 SSE 状态通知（不存 DB），告知已收到请求
	if order.Position == 0 {
		go p.notifyUser(edgeReq, "status", "收到您的消息，马上为您处理...", map[string]any{"stage": "processing_start"})
	} else {
		queueMsg := fmt.Sprintf("收到您的消息，前面还有 %d 个请求，我会尽快处理，完成后通知您。", order.Position)
		go p.notifyUser(edgeReq, "status", queueMsg, map[string]any{"stage": "queued", "position": order.Position})
	}

	// 向服务端发 Queued ACK，关闭服务端的等待（不存 DB）
	go p.sendQuickACK(ctx, edgeReq)

	// 派发给 Worker
	p.orderCache.Dispatch(order)
}

// sendQuickACK 向服务端发送快速确认响应，告知服务端请求已进入本地队列。
// 服务端收到后不保存消息到 DB，只向前端发送 "queued" 事件，前端等待后续 push 通知。
// 状态通知（SSE）由调用方在调用此函数前通过 notifyUser 单独发送。
func (p *Proxy) sendQuickACK(ctx context.Context, req *EdgeRequest) {
	if req.Stream {
		// stream 模式：推送 "queued" chunk，服务端不保存消息直接关闭流
		writer, closer, err := p.streamResponse(ctx)
		if err != nil {
			logger.Warn("[Proxy] sendQuickACK(stream) open failed: %v", err)
			// 降级：non-stream Queued ACK
			p.submitResponse(ctx, &EdgeResponse{
				RequestID: req.RequestID, AgentUUID: req.AgentUUID,
				Success: true, Queued: true, Timestamp: time.Now(),
			})
			return
		}
		_ = writer(&EdgeStreamChunk{
			RequestID: req.RequestID, AgentUUID: req.AgentUUID,
			Type: "queued",
		})
		if err := closer(); err != nil {
			logger.Warn("[Proxy] sendQuickACK(stream) close: %v", err)
		}
	} else {
		// 非 stream：提交 Queued=true，服务端不保存消息
		p.submitResponse(ctx, &EdgeResponse{
			RequestID: req.RequestID, AgentUUID: req.AgentUUID,
			Success: true, Queued: true, Timestamp: time.Now(),
		})
	}
}

// notifyUserSaveDB 调用 /edge/notify（带 save_to_db=true），
// 服务端将消息持久化到 DB 并推送到 SSE，实现异步结果交付。
func (p *Proxy) notifyUserSaveDB(ctx context.Context, req *EdgeRequest, content string, meta map[string]any) {
	if req.SessionUUID == "" {
		return
	}
	payload := map[string]any{
		"agent_uuid":   req.AgentUUID,
		"session_uuid": req.SessionUUID,
		"type":         "result",
		"content":      content,
		"save_to_db":   true,
	}
	if len(meta) > 0 {
		payload["metadata"] = meta
	}
	notifyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, err := p.doRequest(notifyCtx, "POST", "/api/v1/edge/notify", payload)
	if err != nil {
		logger.Warn("[Proxy] notifyUserSaveDB failed: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Warn("[Proxy] notifyUserSaveDB: server returned status %d", resp.StatusCode)
	}
}

// buildLLMMessages 构建 LLM 消息列表，若有 attachments 则对最后一条 user 消息注入多模态内容
// 支持 image（图片）和 file（文档：PDF/Word/TXT 提取文本）
func (p *Proxy) buildLLMMessages(ctx context.Context, req *EdgeRequest) ([]llm.Message, error) {
	messages := make([]llm.Message, 0, len(req.Messages))
	n := len(req.Messages)
	for i, m := range req.Messages {
		msg := llm.Message{Role: m.Role, Content: m.Content}
		// 最后一条 user 消息且带附件时，处理附件
		if i == n-1 && m.Role == "user" && len(req.Attachments) > 0 {
			var atts []EdgeAttachment
			if err := json.Unmarshal(req.Attachments, &atts); err != nil {
				logger.Warn("Request %s: parse attachments failed: %v (ignoring)", req.RequestID, err)
			} else {
				docTexts, imageParts, err := p.fetchAttachmentParts(ctx, atts)
				if err != nil {
					return nil, fmt.Errorf("fetch attachments: %w", err)
				}
				fullText := m.Content
				if len(docTexts) > 0 {
					for _, t := range docTexts {
						fullText = fullText + t
					}
					fullText = strings.TrimSpace(fullText)
				}
				if fullText == "" && len(imageParts) == 0 {
					fullText = "请分析以上内容"
				} else if fullText == "" && len(imageParts) > 0 {
					fullText = "请分析以上图片"
				}
				if len(imageParts) > 0 {
					imageParts = append(imageParts, llm.ContentPart{Type: "text", Text: fullText})
					msg.Content = ""
					msg.ContentParts = imageParts
				} else {
					msg.Content = fullText
				}
			}
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// fetchAttachmentParts 从服务器下载附件，返回文档提取的文本片段和图片 ContentPart 列表
func (p *Proxy) fetchAttachmentParts(ctx context.Context, atts []EdgeAttachment) (docTexts []string, imageParts []llm.ContentPart, err error) {
	for _, a := range atts {
		if a.Token == "" {
			continue
		}
		data, mime, downloadErr := p.DownloadFileByToken(ctx, a.Token)
		if downloadErr != nil {
			logger.Warn("Download attachment token %s failed: %v", a.Token, downloadErr)
			continue
		}
		if a.Type == "file" {
			fileName := a.Name
			if fileName == "" {
				fileName = "document"
			}
			extracted, extErr := docparse.ExtractDocumentText(data, fileName)
			if extErr != nil {
				logger.Warn("Extract document %s failed: %v", a.Token, extErr)
				continue
			}
			if extracted != "" {
				baseName := fileName
				if idx := strings.LastIndex(fileName, "/"); idx >= 0 {
					baseName = fileName[idx+1:]
				} else if idx := strings.LastIndex(fileName, "\\"); idx >= 0 {
					baseName = fileName[idx+1:]
				}
				docTexts = append(docTexts, "\n\n--- 文档内容 ("+baseName+") ---\n"+extracted)
			}
			continue
		}
		if a.Type == "image" {
			if mime == "" {
				mime = "image/jpeg"
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			imageParts = append(imageParts, llm.ContentPart{
				Type:        "image",
				ImageBase64: b64,
				MimeType:    mime,
			})
		}
	}
	return docTexts, imageParts, nil
}

// selectProvider 根据请求中的模型名选择 Provider，优先使用请求指定的，否则使用默认（带 fallback）
func (p *Proxy) selectProvider(requestModel string) (llm.Provider, error) {
	// 优先使用请求中指定的 provider 名称
	if requestModel != "" {
		provider, err := p.llmRegistry.Get(requestModel)
		if err == nil {
			return provider, nil
		}
		logger.Debug("Requested provider %q not found, falling back to default", requestModel)
	}
	// 使用带 fallback 的默认 Provider
	if p.defaultProvider != nil {
		return p.defaultProvider, nil
	}
	return p.llmRegistry.Default()
}

// responseSink 本地测试时用于接收响应，nil 时使用 submitResponse 提交到服务器
type responseSink func(ctx context.Context, resp *EdgeResponse)

// handleRequest 处理单个请求，支持三阶段 Skill 管道和 tool calling 循环
// 流程: Pre Skills → Rules注入 → LLM(+Tool Calling循环) → Post Skills → 响应
// 若 sink 非 nil，将响应交给 sink 而非提交到服务器（用于本地聊天测试）
func (p *Proxy) handleRequest(ctx context.Context, req *EdgeRequest, sink responseSink) {
	submit := func(ctx context.Context, resp *EdgeResponse) {
		if sink != nil {
			sink(ctx, resp)
		} else {
			p.submitResponse(ctx, resp)
		}
	}
	start := time.Now()
	var success bool
	defer func() {
		elapsed := time.Since(start)
		p.statsCollector.RecordRequest(success, elapsed)
		p.publishStats()
	}()

	edgeResp := &EdgeResponse{
		RequestID: req.RequestID,
		AgentUUID: req.AgentUUID,
		Timestamp: time.Now(),
	}

	// 根据请求选择 Provider（优先使用 LLMProvider，其次使用 Model）
	providerName := req.LLMProvider
	if providerName == "" {
		providerName = req.Model
	}
	provider, err := p.selectProvider(providerName)
	if err != nil {
		logger.Error("Request %s: no LLM provider available: %v", req.RequestID, err)
		edgeResp.Success = false
		edgeResp.Error = fmt.Sprintf("no LLM provider available: %v", err)
		submit(ctx, edgeResp)
		return
	}

	// === 阶段 1: Pre-conversation Skills ===
	var preResult *skills.PreResult
	if p.skillPipeline != nil {
		preInput := &skills.SkillInput{
			UserMessage: lastUserMessage(req.Messages),
		}
		preResult, err = p.skillPipeline.ExecutePreConversation(ctx, preInput)
		if err != nil {
			logger.Warn("Request %s: pre-conversation skills error: %v (continuing)", req.RequestID, err)
		}
	}

	// 构建 LLM 请求（含 image_upload 附件的多模态支持）
	messages, err := p.buildLLMMessages(ctx, req)
	if err != nil {
		logger.Error("Request %s: build messages failed: %v", req.RequestID, err)
		edgeResp.Success = false
		edgeResp.Error = fmt.Sprintf("failed to process attachments: %v", err)
		submit(ctx, edgeResp)
		return
	}

	// 注入 Rules 上下文到 system prompt
	systemPrompt := req.SystemPrompt
	if p.rulesEngine != nil {
		systemPrompt = p.rulesEngine.InjectIntoSystemPrompt(systemPrompt)
	}

	// 注入 Pre-conversation Skills 产出到 system prompt
	if preResult != nil && preResult.ExtraSystemPrompt != "" {
		if systemPrompt != "" {
			systemPrompt += "\n\n"
		}
		systemPrompt += preResult.ExtraSystemPrompt
	}

	// 注入 MCP Resources 到 system prompt
	if p.resourceMgr != nil {
		systemPrompt = p.resourceMgr.InjectIntoSystemPrompt(ctx, systemPrompt)
	}

	// 注入用户记忆（MemoryEnabled 且 user_id 存在时，调用服务端 API 获取）
	if req.MemoryEnabled && req.UserID != "" {
		memoryFragment := p.fetchMemoryFragment(ctx, req.AgentUUID, req.UserID)
		if memoryFragment != "" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += memoryFragment
		}
	}

	llmReq := &llm.CompletionRequest{
		Messages:     messages,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		SystemPrompt: systemPrompt,
	}

	// === 阶段 2: Mid-conversation (LLM + Tool Calling) ===
	// 添加 tool 定义（如果 ToolExecutor 可用）
	if p.toolExecutor != nil {
		llmReq.Tools = p.toolExecutor.GetToolDefinitions(req.MemoryEnabled)
		logger.Debug("Request %s: %d tools passed to LLM", req.RequestID, len(llmReq.Tools))
	}

	// Tool calling 循环（注入 EdgeReqInfo、memory_actions 收集器供 memory 工具使用）
	var memoryActions []MemoryAction
	toolCtx := context.WithValue(ctx, edgeReqInfoKey{}, &EdgeReqInfo{
		UserID:    req.UserID,
		AgentUUID: req.AgentUUID,
	})
	toolCtx = context.WithValue(toolCtx, memoryActionsKey{}, &memoryActions)

	var totalInputTokens, totalOutputTokens, totalTokens int
	var finalContent string
	var finalModel string

	for round := 0; round <= maxToolCallingRounds; round++ {
		logger.Debug("Request %s: calling LLM provider=%s (round %d)...", req.RequestID, provider.Name(), round)
		llmResp, err := provider.Complete(ctx, llmReq)
		if err != nil {
			logger.Error("Request %s: LLM call failed (provider=%s, round=%d, elapsed %s): %v",
				req.RequestID, provider.Name(), round, time.Since(start), err)
			edgeResp.Success = false
			edgeResp.Error = fmt.Sprintf("LLM error: %v", err)
			submit(ctx, edgeResp)
			p.errorCount++
			return
		}

		totalInputTokens += llmResp.InputTokens
		totalOutputTokens += llmResp.OutputTokens
		totalTokens += llmResp.TotalTokens
		finalModel = llmResp.Model

		// 没有 tool calls 或没有 tool executor，LLM 阶段完成
		if !llmResp.HasToolCalls() || p.toolExecutor == nil {
			if llmResp.HasToolCalls() && p.toolExecutor == nil {
				logger.Warn("Request %s: LLM returned %d tool calls but tool executor is nil", req.RequestID, len(llmResp.ToolCalls))
			}
			finalContent = llmResp.Content
			if round > 0 {
				logger.Info("Request %s: LLM completed with %d rounds", req.RequestID, round+1)
			}
			break
		}

		// 有 tool calls，执行工具并继续循环
		logger.Info("Request %s: LLM requested %d tool calls (round %d): %v",
			req.RequestID, len(llmResp.ToolCalls), round, toolCallNames(llmResp.ToolCalls))

		for _, tc := range llmResp.ToolCalls {
			p.notifyUser(req, "status", fmt.Sprintf("正在调用 %s...", tc.Name),
				map[string]any{"stage": "tool_call", "tool_name": tc.Name})
		}
		toolResults := p.toolExecutor.Execute(toolCtx, llmResp.ToolCalls)

		// 检查 [NOTIFY] 前缀：工具要求直接推送通知给用户并终止循环
		if shouldStop := p.handleToolNotify(ctx, req, toolResults); shouldStop {
			edgeResp.Success = true
			edgeResp.Content = ""
			submit(ctx, edgeResp)
			success = true
			return
		}

		for _, tc := range llmResp.ToolCalls {
			p.notifyUser(req, "status", fmt.Sprintf("%s 执行完成", tc.Name),
				map[string]any{"stage": "tool_done", "tool_name": tc.Name})
		}

		// 将 assistant 消息（含 tool_calls）追加到对话历史
		// OpenAI 要求 tool 消息前必须有包含 tool_calls 的 assistant 消息
		llmReq.Messages = append(llmReq.Messages, llm.Message{
			Role:      "assistant",
			Content:   llmResp.Content,
			ToolCalls: llmResp.ToolCalls,
		})
		llmReq.ToolResults = toolResults

		// 检查是否到最后一轮
		if round == maxToolCallingRounds {
			logger.Warn("Request %s: tool calling exceeded max rounds (%d)", req.RequestID, maxToolCallingRounds)
			edgeResp.Success = false
			edgeResp.Error = fmt.Sprintf("tool calling exceeded max rounds (%d)", maxToolCallingRounds)
			submit(ctx, edgeResp)
			p.errorCount++
			return
		}
	}

	// === 阶段 3: Post-conversation Skills（含 TTS 等 code skill）===
	if p.skillPipeline != nil {
		var extraCtx map[string]interface{}
		if preResult != nil {
			extraCtx = preResult.ExtraContext
		}
		postResult, err := p.skillPipeline.ExecutePostConversation(ctx, finalContent, extraCtx)
		if err != nil {
			logger.Warn("Request %s: post-conversation skills error: %v (using original content)", req.RequestID, err)
		} else {
			finalContent = postResult.Content
			// 从 post skill metadata 中提取音频数据
			if audio, ok := postResult.Metadata["audio_base64"].(string); ok && audio != "" {
				edgeResp.AudioBase64 = audio
			}
			if format, ok := postResult.Metadata["audio_format"].(string); ok && format != "" {
				edgeResp.AudioFormat = format
			}
		}
	}

	// === 大文件上传（替换 base64 为下载 URL） ===
	if edgeResp.AudioBase64 != "" && len(edgeResp.AudioBase64) > 10*1024 {
		audioData, err := base64.StdEncoding.DecodeString(edgeResp.AudioBase64)
		if err == nil {
			audioFormat := edgeResp.AudioFormat
			if audioFormat == "" {
				audioFormat = "mp3"
			}
			filename := fmt.Sprintf("audio_%s.%s", req.RequestID, audioFormat)
			contentType := "audio/" + audioFormat
			result, uploadErr := p.UploadFile(ctx, audioData, filename, contentType)
			if uploadErr == nil {
				edgeResp.AudioURL = result.DownloadURL
				edgeResp.AudioBase64 = ""
				logger.Info("Request %s: audio uploaded, token=%s url=%s", req.RequestID, result.Token, result.DownloadURL)
			} else {
				logger.Warn("Request %s: audio upload failed, falling back to base64: %v", req.RequestID, uploadErr)
			}
		} else {
			logger.Warn("Request %s: failed to decode audio base64: %v", req.RequestID, err)
		}
	}

	// === 提交最终响应 ===
	elapsed := time.Since(start)
	edgeResp.Success = true
	edgeResp.Content = finalContent
	edgeResp.Model = finalModel
	edgeResp.Usage = &EdgeTokenUsage{
		InputTokens:  totalInputTokens,
		OutputTokens: totalOutputTokens,
		TotalTokens:  totalTokens,
	}
	// 注入 memory 操作供 client 展示
	if len(memoryActions) > 0 {
		if edgeResp.Metadata == nil {
			edgeResp.Metadata = make(map[string]any)
		}
		edgeResp.Metadata["memory_actions"] = memoryActions
	}
	submit(ctx, edgeResp)
	success = true // 标记请求成功
	if edgeResp.AudioBase64 != "" {
		logger.Info("Request %s completed: provider=%s model=%s tokens=%d elapsed=%s (with audio)",
			req.RequestID, provider.Name(), finalModel, totalTokens, elapsed)
	} else {
		logger.Info("Request %s completed: provider=%s model=%s tokens=%d elapsed=%s",
			req.RequestID, provider.Name(), finalModel, totalTokens, elapsed)
	}
}

// streamResponse 通过长连接 POST 流式推送响应块到服务器（NDJSON 长连接）
// 返回 writer 函数和 closer 函数，调用者通过 writer 发送 chunk，结束后调用 closer 关闭连接
func (p *Proxy) streamResponse(ctx context.Context) (writer func(chunk *EdgeStreamChunk) error, closer func() error, err error) {
	pr, pw := io.Pipe()

	url := p.config.ServerURL + "/api/v1/edge/stream-respond"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, pr)
	if err != nil {
		pw.Close()
		return nil, nil, fmt.Errorf("failed to create stream request: %w", err)
	}
	httpReq.Header.Set("X-Edge-Token", p.config.EdgeToken)
	httpReq.Header.Set("Content-Type", "application/x-ndjson")
	httpReq.Header.Set("Transfer-Encoding", "chunked")

	streamClient := &http.Client{Timeout: 0}

	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := streamClient.Do(httpReq)
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	writer = func(chunk *EdgeStreamChunk) error {
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		data = append(data, '\n')
		_, err = pw.Write(data)
		return err
	}

	closer = func() error {
		pw.Close()
		select {
		case resp := <-respCh:
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("stream-respond returned status %d", resp.StatusCode)
			}
			return nil
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return writer, closer, nil
}

// handleRequestStream 处理流式请求：使用 StreamComplete + NDJSON 长连接推送
func (p *Proxy) handleRequestStream(ctx context.Context, req *EdgeRequest) {
	start := time.Now()
	var success bool
	defer func() {
		elapsed := time.Since(start)
		p.statsCollector.RecordRequest(success, elapsed)
		p.publishStats()
	}()

	providerName := req.LLMProvider
	if providerName == "" {
		providerName = req.Model
	}
	provider, err := p.selectProvider(providerName)
	if err != nil {
		logger.Error("Request %s: no LLM provider available: %v", req.RequestID, err)
		p.submitResponse(ctx, &EdgeResponse{
			RequestID: req.RequestID, AgentUUID: req.AgentUUID,
			Success: false, Error: fmt.Sprintf("no LLM provider available: %v", err),
			Timestamp: time.Now(),
		})
		p.errorCount++
		return
	}

	writer, closer, err := p.streamResponse(ctx)
	if err != nil {
		logger.Error("Request %s: failed to open stream connection: %v", req.RequestID, err)
		p.submitResponse(ctx, &EdgeResponse{
			RequestID: req.RequestID, AgentUUID: req.AgentUUID,
			Success: false, Error: fmt.Sprintf("stream connection failed: %v", err),
			Timestamp: time.Now(),
		})
		p.errorCount++
		return
	}
	defer closer()

	sendError := func(errMsg string) {
		writer(&EdgeStreamChunk{
			RequestID: req.RequestID, AgentUUID: req.AgentUUID,
			Type: "error", Error: errMsg,
		})
	}

	// === Pre-conversation Skills ===
	var preResult *skills.PreResult
	if p.skillPipeline != nil {
		preInput := &skills.SkillInput{UserMessage: lastUserMessage(req.Messages)}
		preResult, err = p.skillPipeline.ExecutePreConversation(ctx, preInput)
		if err != nil {
			logger.Warn("Request %s: pre-conversation skills error: %v (continuing)", req.RequestID, err)
		}
	}

	messages, err := p.buildLLMMessages(ctx, req)
	if err != nil {
		logger.Error("Request %s: build messages failed: %v", req.RequestID, err)
		sendError(fmt.Sprintf("failed to process attachments: %v", err))
		p.errorCount++
		return
	}

	systemPrompt := req.SystemPrompt
	if p.rulesEngine != nil {
		systemPrompt = p.rulesEngine.InjectIntoSystemPrompt(systemPrompt)
	}
	if preResult != nil && preResult.ExtraSystemPrompt != "" {
		if systemPrompt != "" {
			systemPrompt += "\n\n"
		}
		systemPrompt += preResult.ExtraSystemPrompt
	}
	if p.resourceMgr != nil {
		systemPrompt = p.resourceMgr.InjectIntoSystemPrompt(ctx, systemPrompt)
	}
	if req.MemoryEnabled && req.UserID != "" {
		memoryFragment := p.fetchMemoryFragment(ctx, req.AgentUUID, req.UserID)
		if memoryFragment != "" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += memoryFragment
		}
	}

	llmReq := &llm.CompletionRequest{
		Messages:     messages,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		SystemPrompt: systemPrompt,
	}
	if p.toolExecutor != nil {
		llmReq.Tools = p.toolExecutor.GetToolDefinitions(req.MemoryEnabled)
		logger.Info("Request %s: %d tools passed to LLM", req.RequestID, len(llmReq.Tools))
	}

	var memoryActions []MemoryAction
	toolCtx := context.WithValue(ctx, edgeReqInfoKey{}, &EdgeReqInfo{UserID: req.UserID, AgentUUID: req.AgentUUID})
	toolCtx = context.WithValue(toolCtx, memoryActionsKey{}, &memoryActions)

	var totalInputTokens, totalOutputTokens, totalTokens int
	var finalContent string
	var finalModel string

	for round := 0; round <= maxToolCallingRounds; round++ {
		logger.Debug("Request %s: calling LLM provider=%s (round %d, stream)...", req.RequestID, provider.Name(), round)

		streamCh, streamErr := provider.StreamComplete(ctx, llmReq)
		if streamErr != nil {
			logger.Error("Request %s: LLM StreamComplete failed (provider=%s, round=%d): %v",
				req.RequestID, provider.Name(), round, streamErr)
			sendError(fmt.Sprintf("LLM error: %v", streamErr))
			p.errorCount++
			return
		}

		var roundContent strings.Builder
		var roundToolCalls []llm.ToolCall
		var roundModel string

		for event := range streamCh {
			if event.Error != nil {
				logger.Error("Request %s: stream error (round=%d): %v", req.RequestID, round, event.Error)
				sendError(fmt.Sprintf("LLM stream error: %v", event.Error))
				p.errorCount++
				return
			}
			if event.Content != "" {
				roundContent.WriteString(event.Content)
				writer(&EdgeStreamChunk{
					RequestID: req.RequestID, AgentUUID: req.AgentUUID,
					Type: "delta", Content: event.Content,
				})
			}
			if event.Done {
				totalInputTokens += event.InputTokens
				totalOutputTokens += event.OutputTokens
				totalTokens += event.InputTokens + event.OutputTokens
				roundModel = event.Model
				roundToolCalls = event.ToolCalls
			}
		}

		if roundModel != "" {
			finalModel = roundModel
		}

		if len(roundToolCalls) == 0 || p.toolExecutor == nil {
			finalContent = roundContent.String()
			if round > 0 {
				logger.Info("Request %s: LLM stream completed with %d rounds", req.RequestID, round+1)
			}
			break
		}

		toolNames := toolCallNames(roundToolCalls)
		logger.Info("Request %s: LLM requested %d tool calls (round %d, stream): %v",
			req.RequestID, len(roundToolCalls), round, toolNames)

		for _, name := range toolNames {
			p.notifyUser(req, "status", fmt.Sprintf("正在调用 %s...", name),
				map[string]any{"tool_name": name, "stage": "tool_call"})
		}

		toolResults := p.toolExecutor.Execute(toolCtx, roundToolCalls)

		// 检查 [NOTIFY] 前缀：工具要求直接推送通知给用户并终止循环
		if shouldStop := p.handleToolNotify(ctx, req, toolResults); shouldStop {
			writer(&EdgeStreamChunk{
				RequestID: req.RequestID, AgentUUID: req.AgentUUID,
				Type: "done", Model: finalModel,
				Usage: &EdgeTokenUsage{
					InputTokens:  totalInputTokens,
					OutputTokens: totalOutputTokens,
					TotalTokens:  totalTokens,
				},
			})
			success = true
			return
		}

		for _, name := range toolNames {
			p.notifyUser(req, "status", fmt.Sprintf("%s 执行完成", name),
				map[string]any{"tool_name": name, "stage": "tool_done"})
		}

		llmReq.Messages = append(llmReq.Messages, llm.Message{
			Role: "assistant", Content: roundContent.String(), ToolCalls: roundToolCalls,
		})
		llmReq.ToolResults = toolResults

		if round == maxToolCallingRounds {
			logger.Warn("Request %s: tool calling exceeded max rounds (%d)", req.RequestID, maxToolCallingRounds)
			sendError(fmt.Sprintf("tool calling exceeded max rounds (%d)", maxToolCallingRounds))
			p.errorCount++
			return
		}
	}

	// === Post-conversation Skills ===
	if p.skillPipeline != nil {
		var extraCtx map[string]interface{}
		if preResult != nil {
			extraCtx = preResult.ExtraContext
		}
		postResult, err := p.skillPipeline.ExecutePostConversation(ctx, finalContent, extraCtx)
		if err != nil {
			logger.Warn("Request %s: post-conversation skills error: %v (using original content)", req.RequestID, err)
		} else {
			finalContent = postResult.Content
		}
	}

	// 发送 done 事件
	doneChunk := &EdgeStreamChunk{
		RequestID: req.RequestID, AgentUUID: req.AgentUUID,
		Type:  "done",
		Model: finalModel,
		Usage: &EdgeTokenUsage{
			InputTokens:  totalInputTokens,
			OutputTokens: totalOutputTokens,
			TotalTokens:  totalTokens,
		},
	}
	if len(memoryActions) > 0 {
		doneChunk.Metadata = map[string]any{"memory_actions": memoryActions}
	}
	writer(doneChunk)

	success = true
	elapsed := time.Since(start)
	logger.Info("Request %s stream completed: provider=%s model=%s tokens=%d elapsed=%s",
		req.RequestID, provider.Name(), finalModel, totalTokens, elapsed)
}

// notifyCaptureFn 是注入到 context 中的回调 key，供 Worker pool 捕获 [NOTIFY] 内容。
type notifyCaptureFn struct{}

// handleToolNotify 扫描工具执行结果中的 [NOTIFY] 前缀。
// 若发现，立即通过 SSE 推送给用户，并返回 true 表示应终止 LLM 循环。
// 若 ctx 中注入了 notifyCaptureFn 回调（Worker pool 场景），还会额外调用回调，
// 让调用方有机会将内容持久化到 DB。
func (p *Proxy) handleToolNotify(ctx context.Context, req *EdgeRequest, results []llm.ToolResult) (shouldStop bool) {
	const prefix = "[NOTIFY]"
	for i, r := range results {
		if strings.HasPrefix(r.Content, prefix) {
			msg := strings.TrimPrefix(r.Content, prefix)
			results[i].Content = msg
			shouldStop = true
			// 始终通过 SSE 发送即时通知（不存 DB）
			p.notifyUser(req, "status", msg, nil)
			// Worker pool 场景：通过 context 回调捕获内容，稍后存入 DB
			if cb, ok := ctx.Value(notifyCaptureFn{}).(func(string)); ok {
				cb(msg)
			}
		}
	}
	return
}

// notifyUser 向用户推送 Edge 状态通知（异步、失败静默忽略，不影响主流程）
func (p *Proxy) notifyUser(req *EdgeRequest, notifyType, content string, meta map[string]any) {
	if req.SessionUUID == "" {
		return
	}
	go func() {
		payload := map[string]any{
			"agent_uuid":   req.AgentUUID,
			"session_uuid": req.SessionUUID,
			"type":         notifyType,
			"content":      content,
		}
		if len(meta) > 0 {
			payload["metadata"] = meta
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := p.doRequest(ctx, "POST", "/api/v1/edge/notify", payload)
		if err != nil {
			logger.Warn("notifyUser: request failed: %v", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			logger.Warn("notifyUser: server returned status %d", resp.StatusCode)
		}
	}()
}

// lastUserMessage 获取最后一条用户消息的内容
func toolCallNames(calls []llm.ToolCall) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
	}
	return names
}

func lastUserMessage(messages []EdgeMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// submitResponse 提交响应到服务器（带重试）
func (p *Proxy) submitResponse(ctx context.Context, resp *EdgeResponse) {
	maxAttempts := 3
	delay := 1 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		httpResp, err := p.doRequest(ctx, "POST", "/api/v1/edge/respond", resp)
		if err != nil {
			logger.Warn("Submit response %s attempt %d/%d failed: %v", resp.RequestID, attempt, maxAttempts, err)
			if attempt < maxAttempts {
				time.Sleep(delay)
				delay *= 2
			}
			continue
		}
		httpResp.Body.Close()
		if httpResp.StatusCode == http.StatusOK {
			logger.Debug("Response %s submitted successfully", resp.RequestID)
			return
		}
		logger.Warn("Submit response %s returned status %d", resp.RequestID, httpResp.StatusCode)
		if attempt < maxAttempts {
			time.Sleep(delay)
			delay *= 2
		}
	}
	logger.Error("Failed to submit response %s after %d attempts", resp.RequestID, maxAttempts)
}

// CompleteLocal 在本地执行一次完整请求（Pre/Mid/Post Skills + LLM + Tool Calling），不提交到服务器。
// 用于 TUI 聊天界面本地测试 Skill 与对话流程。ctx 超时控制整体执行时间。
func (p *Proxy) CompleteLocal(ctx context.Context, req *EdgeRequest) (*EdgeResponse, error) {
	ch := make(chan *EdgeResponse, 1)
	sink := func(_ context.Context, resp *EdgeResponse) {
		select {
		case ch <- resp:
		default:
		}
	}
	go p.handleRequest(ctx, req, sink)
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// edgeMemoryItem 服务端返回的 memory 项（仅解析需要的字段）
type edgeMemoryItem struct {
	Content string `json:"content"`
}

// fetchMemoryFragment 调用服务端 Edge Memory API 获取用户记忆，格式化为 system prompt 片段
func (p *Proxy) fetchMemoryFragment(ctx context.Context, agentUUID, userID string) string {
	path := fmt.Sprintf("/api/v1/edge/memories?agent_uuid=%s&user_id=%s",
		url.QueryEscape(agentUUID), url.QueryEscape(userID))
	resp, err := p.doRequest(ctx, "GET", path, nil)
	if err != nil {
		logger.Warn("Fetch memories failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Fetch memories returned status %d", resp.StatusCode)
		return ""
	}

	// 服务端 RespondJSON 包装为 {"success": true, "data": [...]}
	var envelope struct {
		Success bool              `json:"success"`
		Data    json.RawMessage   `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		logger.Warn("Parse memories response failed: %v", err)
		return ""
	}
	var memories []edgeMemoryItem
	if err := json.Unmarshal(envelope.Data, &memories); err != nil {
		logger.Warn("Parse memories data failed: %v", err)
		return ""
	}

	if len(memories) == 0 {
		return ""
	}

	const maxMemories = 50
	if len(memories) > maxMemories {
		memories = memories[:maxMemories]
	}

	fragment := "--- 用户记忆（跨会话有效） ---\n以下是你已记住的关于该用户的信息，请在回复中自然运用，无需重复提及这些信息来源：\n"
	for i, m := range memories {
		fragment += fmt.Sprintf("%d. %s\n", i+1, m.Content)
	}
	fragment += "---"
	return fragment
}

// SaveMemory 实现 EdgeMemoryAPI，调用服务端 API 创建 memory
func (p *Proxy) SaveMemory(ctx context.Context, userID, agentUUID, content, category string) (string, error) {
	body := map[string]interface{}{
		"user_id":  userID,
		"content":  content,
		"category": category,
	}
	resp, err := p.doRequest(ctx, "POST", "/api/v1/edge/memories", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("save memory failed: status %d, body %s", resp.StatusCode, string(b))
	}
	return "已记住", nil
}

// DeleteMemoryByKeyword 实现 EdgeMemoryAPI，调用服务端 API 按关键词删除 memory
func (p *Proxy) DeleteMemoryByKeyword(ctx context.Context, userID, agentUUID, keyword string) (string, error) {
	body := map[string]interface{}{
		"user_id":          userID,
		"content_keyword":  keyword,
	}
	resp, err := p.doRequest(ctx, "POST", "/api/v1/edge/memories/delete-by-keyword", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("delete memory failed: status %d, body %s", resp.StatusCode, string(b))
	}
	var result struct {
		Message string `json:"message"`
		Deleted int    `json:"deleted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "已删除", nil
	}
	if result.Deleted > 0 {
		return fmt.Sprintf("已删除 %d 条相关记忆", result.Deleted), nil
	}
	return "没有找到匹配的记忆", nil
}

// doRequest 发送 HTTP 请求到 Linkyun Server
func (p *Proxy) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := p.config.ServerURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Edge-Token", p.config.EdgeToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return p.httpClient.Do(req)
}
