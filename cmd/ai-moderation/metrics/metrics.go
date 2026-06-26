// Package metrics 提供 ai-moderation 服务的 Prometheus 指标注册。
//
// 指标设计：
//   - ai_moderation_calls_total{result}    counter：调用次数（按 result 分桶 pass/review/block/fallback）
//   - ai_moderation_latency_seconds        histogram：推理延迟（秒）
//   - ai_moderation_circuit_state          gauge：客户端熔断器状态（0=closed, 1=open, 2=half_open）
//   - ai_moderation_service_start_total    counter：服务启动次数
//
// 客户端侧熔断器状态由 pkg/aiclient 写入；服务端本包负责注册与暴露。
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CallsTotal 按结果统计的 AI 调用次数
	CallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_moderation_calls_total",
			Help: "Total AI moderation calls grouped by result (pass/review/block/fallback).",
		},
		[]string{"result"},
	)

	// LatencySeconds 推理延迟分布（秒）
	LatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_moderation_latency_seconds",
			Help:    "AI moderation inference latency in seconds.",
			Buckets: []float64{0.05, 0.1, 0.2, 0.3, 0.5, 0.8, 1.0, 1.5, 2.0},
		},
		[]string{"mode"}, // sync / async
	)

	// CircuitState 客户端熔断器状态
	CircuitState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_moderation_circuit_state",
			Help: "Client-side circuit breaker state (0=closed, 1=open, 2=half_open).",
		},
		[]string{"client"},
	)

	// ServiceStartTotal 服务启动次数
	ServiceStartTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "ai_moderation_service_start_total",
			Help: "Total ai-moderation service start count.",
		},
	)
)

// RecordServiceStart 在 main 启动时自增一次。
func RecordServiceStart() {
	ServiceStartTotal.Inc()
}