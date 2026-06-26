package database

import (
	"errors"
	"time"

	"go_projects/praProject1/cmd/content/model"

	"gorm.io/gorm"
)

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
func CreateAIAuditLog(db *gorm.DB, log *model.AIAuditLog) error {
	if log == nil {
		return errors.New("log is nil")
	}
	if log.PostID <= 0 {
		return errors.New("post_id must be > 0")
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	return db.Create(log).Error
}

// ListAIAuditLogsByPostID 查询某帖子的 AI 审核日志（按时间倒序）
//
// 参数：
//   - postID: 帖子 ID
//   - limit: 返回条数限制（最大 100）
func ListAIAuditLogsByPostID(db *gorm.DB, postID int64, limit int) ([]model.AIAuditLog, error) {
	if postID <= 0 {
		return nil, errors.New("post_id must be > 0")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var logs []model.AIAuditLog
	err := db.Where("post_id = ?", postID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// CountAIAuditLogsByStatus 统计指定状态的 AI 审计日志数量（用于监控降级率）
func CountAIAuditLogsByStatus(db *gorm.DB, status model.AIStatus, since time.Time) (int64, error) {
	var count int64
	err := db.Model(&model.AIAuditLog{}).
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
func CleanupOldAIAuditLogs(db *gorm.DB, before time.Time) (int64, error) {
	result := db.Where("created_at < ?", before).Delete(&model.AIAuditLog{})
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
func GetAIStatusDistribution(db *gorm.DB, since time.Time) (*AIStatusDistribution, error) {
	dist := &AIStatusDistribution{}
	rows, err := db.Model(&model.AIAuditLog{}).
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