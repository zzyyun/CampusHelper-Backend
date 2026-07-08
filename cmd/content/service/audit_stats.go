// Package service - audit_stats.go 提供 AI 审核统计分析接口。
//
// 功能（PRD § Success Metrics）：
//   - Dashboard 概览：总调用数、通过率、拦截率、人工率、平均延迟
//   - 每日趋势：近 N 天的审核量/拦截量趋势
//   - 类别分布：命中违规类别的频次统计
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md § Success Metrics
//   - 任务 task-005
package service

import (
	"time"

	"go_projects/praProject1/cmd/content/database"
	"go_projects/praProject1/cmd/content/model"
)

// AuditStatsService AI 审核统计分析服务
type AuditStatsService struct{}

// NewAuditStatsService 创建统计分析服务
func NewAuditStatsService() *AuditStatsService {
	return &AuditStatsService{}
}

// DashboardStats Dashboard 概览数据
type DashboardStats struct {
	// 总量
	TotalCalls   int64   `json:"total_calls"`   // 总审核调用数
	TotalPass    int64   `json:"total_pass"`    // PASS 数量
	TotalReview  int64   `json:"total_review"`  // REVIEW 数量（进人工池）
	TotalBlock   int64   `json:"total_block"`   // BLOCK 数量（自动拦截）

	// 比率（百分比，保留 2 位小数）
	PassRate   float64 `json:"pass_rate"`   // AI 自动放行率
	BlockRate  float64 `json:"block_rate"`  // AI 自动拦截率
	ReviewRate float64 `json:"review_rate"` // 进人工池率

	// 性能
	AvgLatencyMs float64 `json:"avg_latency_ms"` // 平均推理延迟（ms）

	// 状态分布
	StatusDist StatusDistribution `json:"status_dist"` // SYNCED/DEGRADED/ASYNC 分布
}

// StatusDistribution AI 调用状态分布
type StatusDistribution struct {
	Synced   int64 `json:"synced"`   // 正常同步调用
	Degraded int64 `json:"degraded"` // 降级调用
	Async    int64 `json:"async"`    // 异步补判调用
}

// DailyTrend 每日趋势数据点
type DailyTrend struct {
	Date       string `json:"date"`        // 日期 (YYYY-MM-DD)
	TotalCalls int64  `json:"total_calls"` // 当日总调用
	PassCount  int64  `json:"pass_count"`  // 当日 PASS
	BlockCount int64  `json:"block_count"` // 当日 BLOCK
	ReviewCount int64 `json:"review_count"` // 当日 REVIEW
}

// CategoryDistribution 违规类别分布
type CategoryDistribution struct {
	Category string `json:"category"` // 类别名称
	Count    int64  `json:"count"`    // 出现次数
}

// GetDashboardStats 获取 Dashboard 概览数据
//
// 参数 since: 统计起始时间（如 7 天前）
func (s *AuditStatsService) GetDashboardStats(since time.Time) (*DashboardStats, error) {
	// 获取状态分布
	dist, err := database.GetAIStatusDistribution(since)
	if err != nil {
		return nil, err
	}

	// 获取各 AI 结果的数量
	passCount, err := database.CountAIAuditLogsByResult(model.AIResult(0), since) // PASS=0
	if err != nil {
		return nil, err
	}
	reviewCount, err := database.CountAIAuditLogsByResult(model.AIResult(1), since) // REVIEW=1
	if err != nil {
		return nil, err
	}
	blockCount, err := database.CountAIAuditLogsByResult(model.AIResult(2), since) // BLOCK=2
	if err != nil {
		return nil, err
	}

	total := passCount + reviewCount + blockCount
	if total == 0 {
		return &DashboardStats{
			StatusDist: StatusDistribution{
				Synced:   dist.Synced,
				Degraded: dist.Degraded,
				Async:    dist.Async,
			},
		}, nil
	}

	// 计算比率
	passRate := float64(passCount) / float64(total) * 100
	blockRate := float64(blockCount) / float64(total) * 100
	reviewRate := float64(reviewCount) / float64(total) * 100

	// 获取平均延迟
	avgLatency, err := database.GetAIAvgLatency(since)
	if err != nil {
		avgLatency = 0 // 查询失败时返回 0
	}

	return &DashboardStats{
		TotalCalls:   total,
		TotalPass:    passCount,
		TotalReview:  reviewCount,
		TotalBlock:   blockCount,
		PassRate:     passRate,
		BlockRate:    blockRate,
		ReviewRate:   reviewRate,
		AvgLatencyMs: avgLatency,
		StatusDist: StatusDistribution{
			Synced:   dist.Synced,
			Degraded: dist.Degraded,
			Async:    dist.Async,
		},
	}, nil
}

// GetDailyTrend 获取近 N 天的每日审核趋势
func (s *AuditStatsService) GetDailyTrend(days int) ([]database.DailyTrendRow, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	return database.GetAIDailyTrend(time.Now().AddDate(0, 0, -days))
}

// GetCategoryDistribution 获取违规类别分布
func (s *AuditStatsService) GetCategoryDistribution(since time.Time, limit int) ([]database.CategoryDistributionRow, error) {
	if limit <= 0 {
		limit = 10
	}
	return database.GetAICategoryDistribution(since, limit)
}
