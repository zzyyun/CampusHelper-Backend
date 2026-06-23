package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ─── 枚举定义 ──────────────────────────────────────────────────────────────────

// PostType 帖子类型枚举。
type PostType int8

const (
	PostTypeGeneral    PostType = 1 // 通用帖子
	PostTypeLostFound  PostType = 2 // 失物招领
	PostTypeSecondHand PostType = 3 // 二手交易
)

// IsValid 校验 PostType 是否为合法值。
func (t PostType) IsValid() bool {
	return t == PostTypeGeneral || t == PostTypeLostFound || t == PostTypeSecondHand
}

// PostStatus 帖子状态枚举。
type PostStatus int8

const (
	PostStatusPending   PostStatus = 1 // 审核中
	PostStatusPublished PostStatus = 2 // 已发布
	PostStatusExpired   PostStatus = 3 // 已过期
	PostStatusClosed    PostStatus = 4 // 已关闭
	PostStatusRejected  PostStatus = 5 // 已拒绝
	PostStatusRetrieved PostStatus = 6 // 失物已当领（失物招领专用）
	PostStatusSold      PostStatus = 7 // 二手已售出（二手交易专用）
)

// IsValid 校验 PostStatus 是否为合法值。
func (s PostStatus) IsValid() bool {
	switch s {
	case PostStatusPending, PostStatusPublished, PostStatusExpired,
		PostStatusClosed, PostStatusRejected, PostStatusRetrieved, PostStatusSold:
		return true
	}
	return false
}

// ─── StringArray：JSON 数组类型（用于 images 字段）────────────────────────────

// StringArray 字符串数组，序列化为 JSON 存储于 MySQL JSON 字段。
type StringArray []string

// Value 实现 driver.Valuer，将数组序列化为 JSON 字符串。
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner，从 JSON 字符串反序列化为数组。
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("StringArray: unsupported scan type")
	}
	if len(data) == 0 {
		*s = []string{}
		return nil
	}
	return json.Unmarshal(data, s)
}

// ─── Post 主模型 ────────────────────────────────────────────────────────────────

// Post 通用帖子数据模型。
// 对应 MySQL 表 posts，所有业务模板（失物招领、二手交易）共享此表。
// 业务扩展字段（location、price 等）存于独立的扩展表（lost_found_posts / second_hand_posts）。
type Post struct {
	ID           int64          `gorm:"primaryKey;autoIncrement:false" json:"id"` // 雪花 ID
	SchoolID     int64          `gorm:"column:school_id;not null;index" json:"school_id"`
	UserID       int64          `gorm:"column:user_id;not null;index" json:"user_id"`
	Type         PostType       `gorm:"column:type;not null" json:"type"`
	Title        string         `gorm:"size:200;not null" json:"title"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	Images       StringArray    `gorm:"type:json" json:"images"`
	Status       PostStatus     `gorm:"not null;default:1" json:"status"`
	LikesCount   int32          `gorm:"column:likes_count;default:0" json:"likes_count"`
	CommentCount int32          `gorm:"column:comment_count;default:0" json:"comment_count"`
	CreatedAt    time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"not null" json:"updated_at"`
	ExpiredAt    *time.Time     `gorm:"column:expired_at" json:"expired_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名。
func (Post) TableName() string { return "posts" }

// ─── 状态机校验 ────────────────────────────────────────────────────────────────

// allowedTransitions 定义合法的状态流转关系。
// key: 当前状态，value: 允许流转到的状态列表。
var allowedTransitions = map[PostStatus][]PostStatus{
	PostStatusPending: {
		PostStatusPublished, // 审核通过
		PostStatusRejected,  // 审核拒绝
		PostStatusClosed,    // 用户主动撤回
	},
	PostStatusPublished: {
		PostStatusExpired,   // 自然过期
		PostStatusClosed,    // 用户关闭
		PostStatusRetrieved, // 失物已当领（业务扩展）
		PostStatusSold,      // 二手已售出（业务扩展）
	},
}

// CanTransitionTo 校验 from → to 是否为合法的状态流转。
// 非法流转返回 false（如 pending → expired 等）。
func CanTransitionTo(from, to PostStatus) bool {
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}