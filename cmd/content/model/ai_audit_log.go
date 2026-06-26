package model

import (
	"time"
)

// AIResult AI 审核结果枚举（与 PB/pb/ai_moderation_pb.ModerateTextResponse.Result 一致）
type AIResult int8

const (
	AIResultPass   AIResult = 0 // 内容正常，放行
	AIResultReview AIResult = 1 // AI 不确定，进人工池
	AIResultBlock  AIResult = 2 // 内容违规，拦截
)

// String AIResult 的可读字符串
func (r AIResult) String() string {
	switch r {
	case AIResultPass:
		return "pass"
	case AIResultReview:
		return "review"
	case AIResultBlock:
		return "block"
	default:
		return "unknown"
	}
}

// AIStatus AI 调用状态（与 pb.ModerateTextResponse.Status 一致）
type AIStatus int8

const (
	AIStatusSynced   AIStatus = 0 // 正常同步调用
	AIStatusDegraded AIStatus = 1 // AI 服务降级（不可用/超时/熔断）
	AIStatusAsync    AIStatus = 2 // 异步补判调用
)

// String AIStatus 的可读字符串
func (s AIStatus) String() string {
	switch s {
	case AIStatusSynced:
		return "synced"
	case AIStatusDegraded:
		return "degraded"
	case AIStatusAsync:
		return "async"
	default:
		return "unknown"
	}
}

// AIAuditLog AI 审核审计日志，记录每次 AI 调用的完整决策信息。
//
// 与 Content Service posts 表同库（content_db），通过 GORM AutoMigrate 自动创建。
// 与 user-service 的 admin_audit_logs 不同：本表关注 AI 决策，admin_audit_logs 关注管理员操作。
//
// 字段设计：
//   - content_hash: SHA256(text)，不存原始文本（隐私保护）
//   - ai_status: 区分同步/降级/异步，便于审计 AI 服务质量
//   - fallback_used: 标识是否走降级逻辑
//   - trace_id: 关联 Jaeger span
//
// 索引：
//   - idx_post_id: 按 post_id 查询某帖子的 AI 决策历史
//   - idx_created_at: 按时间清理（180 天保留）
//   - idx_ai_status: 按状态统计（降级率、异步命中率）
type AIAuditLog struct {
	ID            int64     `gorm:"primaryKey;autoIncrement:false" json:"id"` // 雪花算法生成
	PostID        int64     `gorm:"column:post_id;index;not null"   json:"post_id"`
	ContentHash   string    `gorm:"size:64;not null;index"           json:"content_hash"` // SHA256(text)
	AIStatus      AIStatus  `gorm:"column:ai_status;not null;index"  json:"ai_status"`
	AIResult      AIResult  `gorm:"column:ai_result;not null"        json:"ai_result"`
	AIConfidence  float32   `gorm:"column:ai_confidence;not null"    json:"ai_confidence"`
	AICategories  string    `gorm:"column:ai_categories;type:text"   json:"ai_categories"` // JSON 数组
	LatencyMs     int64     `gorm:"column:latency_ms;not null"       json:"latency_ms"`
	ModelVersion  string    `gorm:"size:32;not null"                 json:"model_version"`
	FallbackUsed  bool      `gorm:"column:fallback_used;not null;default:false" json:"fallback_used"`
	TraceID       string    `gorm:"size:64"                          json:"trace_id"`
	CreatedAt     time.Time `gorm:"index"                            json:"created_at"`
}

// TableName 指定数据库表名
func (AIAuditLog) TableName() string { return "ai_audit_logs" }