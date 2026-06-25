package model

import (
	"time"

	"gorm.io/gorm"
)

// NotificationType 通知类型常量
type NotificationType string

const (
	NotifLiked        NotificationType = "liked"          // 点赞通知
	NotifPublished    NotificationType = "published"      // 审核通过通知
	NotifReviewResult NotificationType = "review_result"  // 审核拒绝通知
	NotifTakenDown    NotificationType = "taken_down"     // 违规下架通知
	NotifReplied      NotificationType = "replied"        // 评论回复通知
)

// Notification 站内通知数据模型。
// 存储在独立数据库 campus_message 的 notifications 表。
// 标题字段 title 持久化快照（含用户昵称和帖子标题），即使后续昵称修改或帖子删除，通知内容不受影响。
type Notification struct {
	ID         int64          `gorm:"primaryKey;autoIncrement:false"                    json:"id"`
	SchoolID   int64          `gorm:"column:school_id;index;not null"                   json:"school_id"`
	UserID     int64          `gorm:"column:user_id;index:idx_user_read;not null"       json:"user_id"`
	Type       string         `gorm:"size:32;not null"                                  json:"type"`
	Title      string         `gorm:"size:255;not null"                                 json:"title"`
	Content    string         `gorm:"size:500;default:''"                               json:"content"`
	FromUserID int64          `gorm:"column:from_user_id;default:0"                     json:"from_user_id"`
	RefType    string         `gorm:"column:ref_type;size:32;default:''"                json:"ref_type"`
	RefID      int64          `gorm:"column:ref_id;default:0"                           json:"ref_id"`
	IsRead     bool           `gorm:"column:is_read;default:false;index:idx_user_read"  json:"is_read"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index:idx_cleanup"                                 json:"-"`
}

func (Notification) TableName() string { return "notifications" }
