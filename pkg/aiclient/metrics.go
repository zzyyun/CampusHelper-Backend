// Package aiclient 提供 AI 审核的可观测性指标。
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 任务 task-047 (#97)
//
// 暴露的 Prometheus 指标：
//   - ai_client_circuit_state          gauge  客户端熔断器状态（0=closed, 1=open, 2=half_open）
//   - ai_client_calls_total            counter  总调用次数
//   - ai_client_calls_success_total    counter  成功次数
//   - ai_client_calls_failure_total    counter  失败次数
//   - ai_client_calls_fallback_total   counter  降级（fallback）次数
package aiclient

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CircuitState 客户端熔断器状态
	CircuitState = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ai_client_circuit_state",
			Help: "ai-moderation client circuit breaker state (0=closed, 1=open, 2=half_open).",
		},
	)

	// CallsTotal 总调用次数
	CallsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ai_client_calls_total",
			Help: "Total ai-moderation client calls.",
		},
	)

	// CallsSuccess 成功次数
	CallsSuccess = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ai_client_calls_success_total",
			Help: "Successful ai-moderation client calls.",
		},
	)

	// CallsFailure 失败次数（不含 fallback）
	CallsFailure = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ai_client_calls_failure_total",
			Help: "Failed ai-moderation client calls (non-fallback).",
		},
	)

	// CallsFallback 降级次数（fallback）
	CallsFallback = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ai_client_calls_fallback_total",
			Help: "AI moderation fallback calls (DEGRADED mode).",
		},
	)
)

// UpdateCircuitState 更新熔断器状态指标（供 CircuitBreaker.State 变更回调使用）
func UpdateCircuitState(state string) {
	switch state {
	case "closed":
		CircuitState.Set(0)
	case "open":
		CircuitState.Set(1)
	case "half-open":
		CircuitState.Set(2)
	}
}