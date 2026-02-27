package proxy

import (
	"sync"
	"time"
)

// ProxyStats Proxy 运行统计
type ProxyStats struct {
	Connected       bool
	RequestCount    int
	SuccessCount    int
	ErrorCount      int
	LastRequestTime time.Time
	LastErrorTime   time.Time
	AvgResponseTime time.Duration
}

// StatsCollector 统计收集器（线程安全）
type StatsCollector struct {
	stats         ProxyStats
	mu            sync.RWMutex
	responseTimes []time.Duration
	maxSamples    int
}

// NewStatsCollector 创建新的统计收集器
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		maxSamples:    100,
		responseTimes: make([]time.Duration, 0, 100),
	}
}

// SetConnected 设置连接状态
func (s *StatsCollector) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.Connected = connected
}

// RecordRequest 记录请求统计
func (s *StatsCollector) RecordRequest(success bool, responseTime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.RequestCount++
	s.stats.LastRequestTime = time.Now()

	if success {
		s.stats.SuccessCount++
		s.responseTimes = append(s.responseTimes, responseTime)
		if len(s.responseTimes) > s.maxSamples {
			s.responseTimes = s.responseTimes[1:]
		}
		s.updateAvgResponseTime()
	} else {
		s.stats.ErrorCount++
		s.stats.LastErrorTime = time.Now()
	}
}

// updateAvgResponseTime 更新平均响应时间（需持有锁）
func (s *StatsCollector) updateAvgResponseTime() {
	if len(s.responseTimes) == 0 {
		s.stats.AvgResponseTime = 0
		return
	}
	var sum time.Duration
	for _, d := range s.responseTimes {
		sum += d
	}
	s.stats.AvgResponseTime = sum / time.Duration(len(s.responseTimes))
}

// GetStats 获取当前统计数据（返回副本）
func (s *StatsCollector) GetStats() ProxyStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// IncrementErrorCount 增加错误计数
func (s *StatsCollector) IncrementErrorCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.ErrorCount++
	s.stats.LastErrorTime = time.Now()
}
