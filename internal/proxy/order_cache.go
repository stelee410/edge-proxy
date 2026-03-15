package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"linkyun-edge-proxy/internal/logger"
	_ "modernc.org/sqlite"
)

// OrderStatus 订单生命周期状态
type OrderStatus string

const (
	OrderStatusQueued     OrderStatus = "queued"     // 排队等待
	OrderStatusProcessing OrderStatus = "processing" // 正在处理
	OrderStatusDone       OrderStatus = "done"       // 已完成
	OrderStatusFailed     OrderStatus = "failed"     // 处理失败
	OrderStatusRejected   OrderStatus = "rejected"   // 队列满，已拒绝
)

// Order 本地订单：从服务器拉取的一个用户请求
type Order struct {
	ID          int64
	RequestID   string
	SessionUUID string
	AgentUUID   string
	UserID      string
	Request     *EdgeRequest // 反序列化后的完整请求
	Status      OrderStatus
	Position    int       // 排队位置（0 = 立即处理）
	EnqueuedAt  time.Time
}

// OrderCache 基于 SQLite 的本地订单队列，支持崩溃恢复
type OrderCache struct {
	db         *sql.DB
	mu         sync.Mutex
	maxQueue   int        // 最大排队数（不含正在处理）
	workCh     chan *Order // 内存信号通道，供 Worker 监听
}

// NewOrderCache 打开（或创建）SQLite 数据库，初始化表结构。
func NewOrderCache(dbPath string, maxQueue int) (*OrderCache, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open order cache db: %w", err)
	}
	// SQLite 单写者，关闭连接池竞争
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id   TEXT    UNIQUE NOT NULL,
			session_uuid TEXT    NOT NULL,
			agent_uuid   TEXT    NOT NULL,
			user_id      TEXT    NOT NULL DEFAULT '',
			request_json TEXT    NOT NULL,
			status       TEXT    NOT NULL DEFAULT 'queued',
			position     INTEGER NOT NULL DEFAULT 0,
			enqueued_at  TEXT    NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init orders table: %w", err)
	}

	return &OrderCache{
		db:       db,
		maxQueue: maxQueue,
		workCh:   make(chan *Order, maxQueue+16),
	}, nil
}

// Close 关闭数据库连接
func (oc *OrderCache) Close() {
	oc.db.Close()
}

// countQueued 返回当前排队数（内部，调用者需持有锁）
func (oc *OrderCache) countQueued() int {
	var n int
	oc.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE status='queued'`).Scan(&n)
	return n
}

// CountActive 返回当前排队数（不含处理中）
func (oc *OrderCache) CountQueued() int {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	return oc.countQueued()
}

// IsFull 当排队数 >= maxQueue 时返回 true
func (oc *OrderCache) IsFull() bool {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	return oc.countQueued() >= oc.maxQueue
}

// Add 将新订单写入 SQLite 并返回 Order。
// 调用前应检查 IsFull()。
func (oc *OrderCache) Add(req *EdgeRequest) (*Order, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	position := oc.countQueued()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	now := time.Now()
	res, err := oc.db.Exec(`
		INSERT INTO orders (request_id, session_uuid, agent_uuid, user_id, request_json, status, position, enqueued_at)
		VALUES (?, ?, ?, ?, ?, 'queued', ?, ?)
	`, req.RequestID, req.SessionUUID, req.AgentUUID, req.UserID,
		string(data), position, now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}
	id, _ := res.LastInsertId()

	return &Order{
		ID:          id,
		RequestID:   req.RequestID,
		SessionUUID: req.SessionUUID,
		AgentUUID:   req.AgentUUID,
		UserID:      req.UserID,
		Request:     req,
		Status:      OrderStatusQueued,
		Position:    position,
		EnqueuedAt:  now,
	}, nil
}

// MarkProcessing 将订单标记为处理中
func (oc *OrderCache) MarkProcessing(requestID string) {
	oc.db.Exec(`UPDATE orders SET status='processing' WHERE request_id=?`, requestID)
}

// MarkDone 将订单标记为完成
func (oc *OrderCache) MarkDone(requestID string) {
	oc.db.Exec(`UPDATE orders SET status='done' WHERE request_id=?`, requestID)
}

// MarkFailed 将订单标记为失败
func (oc *OrderCache) MarkFailed(requestID string) {
	oc.db.Exec(`UPDATE orders SET status='failed' WHERE request_id=?`, requestID)
}

// MarkRejected 将订单标记为已拒绝
func (oc *OrderCache) MarkRejected(requestID string) {
	oc.db.Exec(`UPDATE orders SET status='rejected' WHERE request_id=?`, requestID)
}

// LoadPendingOrders 启动时加载上次未完成的订单（queued 或 processing 状态）。
// 返回混合列表，调用方通过 Order.Status 区分：
// - processing：上次崩溃时正在处理，应通知用户重试并标记 failed
// - queued：上次排队中未执行，可恢复继续处理
func (oc *OrderCache) LoadPendingOrders() ([]*Order, error) {
	rows, err := oc.db.Query(`
		SELECT id, request_id, session_uuid, agent_uuid, user_id, request_json, status, position, enqueued_at
		FROM orders
		WHERE status IN ('queued','processing')
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		var reqJSON, statusStr, enqueuedStr string
		if err := rows.Scan(&o.ID, &o.RequestID, &o.SessionUUID, &o.AgentUUID,
			&o.UserID, &reqJSON, &statusStr, &o.Position, &enqueuedStr); err != nil {
			logger.Warn("[OrderCache] scan failed: %v", err)
			continue
		}
		o.Status = OrderStatus(statusStr)
		o.EnqueuedAt, _ = time.Parse(time.RFC3339Nano, enqueuedStr)

		var req EdgeRequest
		if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
			logger.Warn("[OrderCache] unmarshal request %s failed: %v", o.RequestID, err)
			continue
		}
		o.Request = &req
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// AllQueuedOrders 返回当前所有 queued 状态的订单（用于关机时通知）
func (oc *OrderCache) AllQueuedOrders() ([]*Order, error) {
	rows, err := oc.db.Query(`
		SELECT id, request_id, session_uuid, agent_uuid, user_id, request_json, position, enqueued_at
		FROM orders
		WHERE status='queued'
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		var reqJSON, enqueuedStr string
		if err := rows.Scan(&o.ID, &o.RequestID, &o.SessionUUID, &o.AgentUUID,
			&o.UserID, &reqJSON, &o.Position, &enqueuedStr); err != nil {
			continue
		}
		o.Status = OrderStatusQueued
		o.EnqueuedAt, _ = time.Parse(time.RFC3339Nano, enqueuedStr)
		var req EdgeRequest
		if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
			continue
		}
		o.Request = &req
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// WorkCh 返回内存工作通道（Worker 监听此通道获取新任务）
func (oc *OrderCache) WorkCh() <-chan *Order {
	return oc.workCh
}

// Dispatch 将订单推入内存工作通道（非阻塞）
func (oc *OrderCache) Dispatch(o *Order) {
	select {
	case oc.workCh <- o:
	default:
		logger.Warn("[OrderCache] workCh full, order %s will be picked up on next poll", o.RequestID)
	}
}
