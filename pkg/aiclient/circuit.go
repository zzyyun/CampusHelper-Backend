// Package aiclient 提供 AI Moderation 客户端熔断器。
//
// 使用 sony/gobreaker 实现客户端侧熔断：
//   - 30s 滑动窗口内连续 5 次失败 → 触发熔断
//   - 熔断持续 30s → 半开状态
//   - 半开状态放行 1 个请求试探，成功则关闭熔断
//
// 设计理由（PRD rev2 修正 #1）：
//   熔断器必须在 Content Service 客户端侧，不能在 ai-moderation 服务端。
//   因为：
//     - 熔断保护的是"调用方不被慢/错的依赖拖垮"
//     - ai-moderation 自身的健康由 health check + 模型加载校验保证
//     - 多个 Content Service 副本各自维护熔断器，避免单点决策
package aiclient

import (
	"errors"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// ─── 熔断器配置 ──────────────────────────────────────────────────────────────

// CircuitConfig 熔断器配置
type CircuitConfig struct {
	// Name 熔断器名称（用于日志/metrics）
	Name string
	// MaxRequests 半开状态允许的最大请求数（默认 1）
	MaxRequests uint32
	// Interval 滑动窗口周期（默认 30s）
	Interval time.Duration
	// Timeout 熔断持续时间（默认 30s）
	Timeout time.Duration
	// ReadyToTrip 触发熔断的回调函数（nil 使用默认：连续 5 次失败）
	ReadyToTrip func(counts gobreaker.Counts) bool
}

// DefaultCircuitConfig 返回默认熔断器配置
func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		Name:        "ai-moderation",
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	}
}

// ─── 熔断器封装 ──────────────────────────────────────────────────────────────

// CircuitBreaker 包装 gobreaker.CircuitBreaker，提供状态查询与 metrics 输出
type CircuitBreaker struct {
	cb      *gobreaker.CircuitBreaker
	mu      sync.RWMutex
	state   string // closed / open / half-open
	metrics *CircuitMetrics
}

// CircuitMetrics 熔断器指标（供 Prometheus 抓取）
type CircuitMetrics struct {
	// CallsTotal 总调用次数
	CallsTotal int64
	// SuccessesTotal 成功次数
	SuccessesTotal int64
	// FailuresTotal 失败次数
	FailuresTotal int64
	// StateChanges 状态变更次数
	StateChanges int64
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(cfg CircuitConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		metrics: &CircuitMetrics{},
	}

	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: cfg.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			cb.mu.Lock()
			cb.state = to.String()
			cb.metrics.StateChanges++
			cb.mu.Unlock()
			// 同步更新 Prometheus 指标
			UpdateCircuitState(to.String())
		},
	}

	cb.cb = gobreaker.NewCircuitBreaker(settings)
	cb.state = gobreaker.StateClosed.String()
	UpdateCircuitState(cb.state) // 初始化 metric
	return cb
}

// Execute 在熔断器保护下执行 fn
func (c *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	c.mu.Lock()
	c.metrics.CallsTotal++
	c.mu.Unlock()

	result, err := c.cb.Execute(fn)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			c.metrics.FailuresTotal++
			return nil, ErrAIServiceUnavailable
		}
		c.metrics.FailuresTotal++
		return nil, err
	}
	c.metrics.SuccessesTotal++
	return result, nil
}

// IsOpen 判断熔断器是否开启
func (c *CircuitBreaker) IsOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == gobreaker.StateOpen.String()
}

// State 查询当前状态字符串
func (c *CircuitBreaker) State() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GetMetrics 获取熔断器指标快照
func (c *CircuitBreaker) GetMetrics() *CircuitMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &CircuitMetrics{
		CallsTotal:     c.metrics.CallsTotal,
		SuccessesTotal: c.metrics.SuccessesTotal,
		FailuresTotal:  c.metrics.FailuresTotal,
		StateChanges:   c.metrics.StateChanges,
	}
}