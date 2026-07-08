package database

import (
	"errors"
	"log"
	"time"

	"go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/pkg/db"

	"gorm.io/gorm"
)

// mustContentDB 返回 content 服务的 *gorm.DB。
// main.go 必须先执行 db.InitContentDB()，否则本函数会记录 fatal。
func mustContentDB() *gorm.DB {
	d, err := db.GetContentDB()
	if err != nil {
		log.Fatalf("[content-db-dao] 未初始化 content db: %v", err)
	}
	return d
}

// ─── ai_audit_logs 表 CRUD ─────────────────────────────────────────────────

// CreateAIAuditLog 创建 AI 审计日志
//
// 参数：
//   - log: AI 审核日志记录（ID 必须由调用方预先设置，雪花算法生成）
//
// 返回：
//   - error: 数据库错误时返回
//
// 错误处理：调用方应吞掉 error 并记录 WARN 日志（PRD § Feature 5：表写入失败不阻塞发帖）
func CreateAIAuditLog(log *model.AIAuditLog) error {
	if log == nil {
		return errors.New("log is nil")
	}
	if log.PostID <= 0 {
		return errors.New("post_id must be > 0")
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	return mustContentDB().Create(log).Error
}

// ListAIAuditLogsByPostID 查询某帖子的 AI 审核日志（按时间倒序）
//
// 参数：
//   - postID: 帖子 ID
//   - limit: 返回条数限制（最大 100）
func ListAIAuditLogsByPostID(postID int64, limit int) ([]model.AIAuditLog, error) {
	if postID <= 0 {
		return nil, errors.New("post_id must be > 0")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var logs []model.AIAuditLog
	err := mustContentDB().Where("post_id = ?", postID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// CountAIAuditLogsByStatus 统计指定状态的 AI 审计日志数量（用于监控降级率）
func CountAIAuditLogsByStatus(status model.AIStatus, since time.Time) (int64, error) {
	var count int64
	err := mustContentDB().Model(&model.AIAuditLog{}).
		Where("ai_status = ? AND created_at >= ?", status, since).
		Count(&count).Error
	return count, err
}

// CleanupOldAIAuditLogs 清理过期 AI 审计日志（PRD: 180 天保留）
//
// 参数：
//   - before: 清理 created_at < before 的所有记录
//
// 返回：
//   - int64: 清理的记录数
//   - error: 数据库错误
func CleanupOldAIAuditLogs(before time.Time) (int64, error) {
	result := mustContentDB().Where("created_at < ?", before).Delete(&model.AIAuditLog{})
	return result.RowsAffected, result.Error
}

// ─── Statistics（用于 Prometheus 指标 / 监控看板）───────────────────────────────

// AIStatusDistribution AI 状态分布（用于统计）
type AIStatusDistribution struct {
	Synced   int64
	Degraded int64
	Async    int64
	Total    int64
}

// GetAIStatusDistribution 查询指定时间范围内的 AI 状态分布
func GetAIStatusDistribution(since time.Time) (*AIStatusDistribution, error) {
	dist := &AIStatusDistribution{}
	rows, err := mustContentDB().Model(&model.AIAuditLog{}).
		Select("ai_status, COUNT(*) as count").
		Where("created_at >= ?", since).
		Group("ai_status").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status int8
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		dist.Total += count
		switch model.AIStatus(status) {
		case model.AIStatusSynced:
			dist.Synced = count
		case model.AIStatusDegraded:
			dist.Degraded = count
		case model.AIStatusAsync:
			dist.Async = count
		}
	}
	return dist, nil
}

// CountAIAuditLogsByResult 统计指定 AI 结果的数量
func CountAIAuditLogsByResult(result model.AIResult, since time.Time) (int64, error) {
	var count int64
	err := mustContentDB().Model(&model.AIAuditLog{}).
		Where("ai_result = ? AND created_at >= ?", result, since).
		Count(&count).Error
	return count, err
}

// GetAIAvgLatency 获取平均推理延迟（毫秒）
func GetAIAvgLatency(since time.Time) (float64, error) {
	var avg float64
	err := mustContentDB().Model(&model.AIAuditLog{}).
		Select("COALESCE(AVG(latency_ms), 0)").
		Where("created_at >= ?", since).
		Scan(&avg).Error
	return avg, err
}

// DailyTrendRow 每日趋势数据行
type DailyTrendRow struct {
	Date        string `json:"date"`
	TotalCalls  int64  `json:"total_calls"`
	PassCount   int64  `json:"pass_count"`
	BlockCount  int64  `json:"block_count"`
	ReviewCount int64  `json:"review_count"`
}

// GetAIDailyTrend 查询近 N 天的每日审核趋势
func GetAIDailyTrend(since time.Time) ([]DailyTrendRow, error) {
	var rows []DailyTrendRow
	err := mustContentDB().Model(&model.AIAuditLog{}).
		Select("DATE(created_at) as date, COUNT(*) as total_calls, "+
			"SUM(CASE WHEN ai_result=0 THEN 1 ELSE 0 END) as pass_count, "+
			"SUM(CASE WHEN ai_result=2 THEN 1 ELSE 0 END) as block_count, "+
			"SUM(CASE WHEN ai_result=1 THEN 1 ELSE 0 END) as review_count").
		Where("created_at >= ?", since).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// CategoryDistributionRow 类别分布行
type CategoryDistributionRow struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// GetAICategoryDistribution 查询违规类别分布（ai_categories 字段 JSON 解析）
//
// 注：ai_categories 存储为 JSON 数组字符串如 ["toxic","insult"]，
// 使用 MySQL JSON_EXTRACT + JSON_TABLE 解析。
// 若 MySQL 版本不支持 JSON_TABLE，使用 LIKE 模糊匹配。
func GetAICategoryDistribution(since time.Time, limit int) ([]CategoryDistributionRow, error) {
	var rows []CategoryDistributionRow
	// 使用 LIKE 模糊匹配（兼容 MySQL 5.7+）
	// 实际生产环境建议升级到 MySQL 8.0 并使用 JSON_TABLE
	err := mustContentDB().Model(&model.AIAuditLog{}).
		Select("ai_categories as category, COUNT(*) as count").
		Where("ai_categories != '' AND ai_categories IS NOT NULL AND created_at >= ?", since).
		Group("ai_categories").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}