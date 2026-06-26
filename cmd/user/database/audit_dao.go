package user_database

import (
	"time"

	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/pkg/snowflake"
)

// CreateAuditLog 写入一条审计日志。
func CreateAuditLog(operatorID, targetID int64, action model.AuditAction, detail string) error {
	entry := &model.AdminAuditLog{
		ID:         snowflake.GenerateID(),
		OperatorID: operatorID,
		TargetID:   targetID,
		Action:     action,
		Detail:     detail,
	}
	return mustUserDB().Create(entry).Error
}

// ListAuditLogs 查询审计日志，按时间倒序，支持游标分页。
func ListAuditLogs(operatorID int64, action string, startTime, endTime time.Time, cursor int64, pageSize int) ([]model.AdminAuditLog, bool, error) {
	query := mustUserDB().Model(&model.AdminAuditLog{})

	if operatorID > 0 {
		query = query.Where("operator_id = ?", operatorID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if !startTime.IsZero() {
		query = query.Where("created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		query = query.Where("created_at <= ?", endTime)
	}
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}

	query = query.Order("id DESC").Limit(pageSize + 1)

	var logs []model.AdminAuditLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(logs) > pageSize
	if hasMore {
		logs = logs[:pageSize]
	}
	return logs, hasMore, nil
}

// CleanupAuditLogs 物理删除指定时间之前的所有审计日志，返回删除条数。
func CleanupAuditLogs(before time.Time) (int64, error) {
	result := mustUserDB().Where("created_at < ?", before).Delete(&model.AdminAuditLog{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
