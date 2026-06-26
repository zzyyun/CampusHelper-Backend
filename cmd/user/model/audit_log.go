package model

import (
	"time"
)

// AuditAction 审计日志操作类型。
type AuditAction string

const (
	AuditActionBanUser   AuditAction = "ban_user"
	AuditActionUnbanUser AuditAction = "unban_user"
	AuditActionSetRole   AuditAction = "set_role"
	AuditActionAuditPost AuditAction = "audit_content"
)

// AdminAuditLog 管理员操作审计日志，记录所有管理类关键操作。
//
// 与 User Service 共用同一个 MySQL 数据库，通过 GORM AutoMigrate 自动创建。
// 日志保留 90 天，超过 90 天的记录由后台清理 goroutine 物理删除。
type AdminAuditLog struct {
	ID         int64       `gorm:"primaryKey;autoIncrement:false" json:"id"` // 雪花算法生成
	OperatorID int64       `gorm:"column:operator_id;index;not null"   json:"operator_id"` // 操作人 user_id
	TargetID   int64       `gorm:"column:target_id;not null"           json:"target_id"`   // 目标 user_id 或 content_id
	Action     AuditAction `gorm:"size:32;not null;index"              json:"action"`      // 操作类型
	Detail     string      `gorm:"type:text;default:''"                json:"detail"`     // JSON 格式详情
	CreatedAt  time.Time   `json:"created_at"`
}

func (AdminAuditLog) TableName() string { return "admin_audit_logs" }
