package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// PostStatus 帖子状态（与 pb.PostStatus 枚举值一一对应）
type PostStatus int8

const (
	PostStatusUnspecified PostStatus = 0
	PostStatusPending     PostStatus = 1 // 审核中
	PostStatusPublished   PostStatus = 2 // 已发布
	PostStatusExpired     PostStatus = 3 // 已过期
	PostStatusClosed      PostStatus = 4 // 已关闭
	PostStatusRejected    PostStatus = 5 // 已拒绝
	PostStatusRetrieved   PostStatus = 6 // 失物已认领（失物招领专用）
	PostStatusSold        PostStatus = 7 // 二手已售出（二手交易专用）
)

// allowedTransitions 定义状态机合法转移图。
// 关键安全设计：任何未列出的转移都视为非法（例如 pending → sold）。
// 这避免了审核未通过就被标记成售出的越权操作。
var allowedTransitions = map[PostStatus]map[PostStatus]struct{}{
	PostStatusPending: {
		PostStatusPublished: {}, // 审核通过
		PostStatusRejected:  {}, // 审核拒绝
	},
	PostStatusPublished: {
		PostStatusClosed:    {}, // 违规下架
		PostStatusExpired:   {}, // 自然过期
		PostStatusRetrieved: {}, // 失物招领 → 已认领
		PostStatusSold:      {}, // 二手交易 → 已售出
	},
}

// CanTransitionTo 校验状态机转移是否合法。
// 返回 nil 表示合法；返回 ErrInvalidTransition 表示非法。
func (s PostStatus) CanTransitionTo(next PostStatus) error {
	if s == next {
		return nil // 幂等操作视为合法
	}
	allowed, ok := allowedTransitions[s]
	if !ok {
		return ErrInvalidTransition
	}
	if _, ok := allowed[next]; !ok {
		return ErrInvalidTransition
	}
	return nil
}

// ErrInvalidTransition 状态机非法转移错误
var ErrInvalidTransition = errors.New("post: 非法的状态转移")

// ─── 业务模型 ────────────────────────────────────────────────────────────────

// Post 帖子主表
// 严格遵循 docs/content-service-prd.md §3.1
type Post struct {
	// ID 由雪花算法生成（pkg/snowflake），关闭数据库自增以避免应用层赋值被覆盖
	ID          int64          `gorm:"primaryKey;autoIncrement:false"        json:"id"`
	SchoolID    int64          `gorm:"column:school_id;not null;index"      json:"school_id"` // 多租户隔离键
	UserID      int64          `gorm:"column:user_id;not null;index"        json:"user_id"`   // 发帖人
	Type        int8           `gorm:"not null;default:1"                   json:"type"`      // 1=通用 2=失物招领 3=二手
	Title       string         `gorm:"size:200;not null"                    json:"title"`
	Content     string         `gorm:"type:text;not null"                   json:"content"`
	ImagesJSON  string         `gorm:"column:images;type:json"              json:"images_json"` // 图片 URL 数组，JSON 存储
	Status      PostStatus     `gorm:"not null;default:1;index"             json:"status"`
	LikesCount   int32          `gorm:"column:likes_count;not null;default:0" json:"likes_count"`
	CommentCount int32          `gorm:"column:comment_count;not null;default:0" json:"comment_count"`
	ExpiredAt   *time.Time     `gorm:"column:expired_at"                    json:"expired_at,omitempty"`

	// ─── 业务扩展字段（按 type 取对应字段） ───────────────────────────────────
	// 失物招领
	LFType        int8    `gorm:"column:lf_type;default:0"       json:"lf_type"`        // 1=lost 2=found
	LFLocation    string  `gorm:"column:lf_location;size:200"    json:"lf_location"`    // 丢失/拾取地点
	LFContact     string  `gorm:"column:lf_contact;size:128"     json:"lf_contact"`     // 联系方式（私密，不索引）
	LFCategory    int8    `gorm:"column:lf_category;default:0"   json:"lf_category"`    // 物品分类
	// 二手交易
	SHPrice       float64 `gorm:"column:sh_price;default:0"      json:"sh_price"`        // 期望售价（元）
	SHOriginal    float64 `gorm:"column:sh_original_price;default:0" json:"sh_original_price"` // 原价
	SHCondition   int8    `gorm:"column:sh_condition;default:0"  json:"sh_condition"`    // 成色 1-4
	SHTradeMethod int8    `gorm:"column:sh_trade_method;default:0" json:"sh_trade_method"` // 交易方式 1=面交 2=快递
	SHCategory    int8    `gorm:"column:sh_category;default:0"   json:"sh_category"`

	// ─── 审核相关字段 ───────────────────────────────────────────────────────────
	RejectReason string     `gorm:"column:reject_reason;size:500"     json:"reject_reason"`  // 审核拒绝/下架原因
	ReviewerID   int64      `gorm:"column:reviewer_id;default:0"      json:"reviewer_id"`    // 审核员 ID
	ReviewedAt   *time.Time `gorm:"column:reviewed_at"                json:"reviewed_at,omitempty"` // 审核时间

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Post) TableName() string { return "posts" }

// PostLike 帖子点赞记录
// 独立成表而非计数器自增，方便未来扩展点赞用户列表/时间线
type PostLike struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	SchoolID  int64     `gorm:"column:school_id;not null;index:idx_post_user,priority:1,unique" json:"school_id"`
	PostID    int64     `gorm:"column:post_id;not null;index:idx_post_user,priority:2,unique"     json:"post_id"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_post_user,priority:3,unique"     json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (PostLike) TableName() string { return "post_likes" }

// PostComment 帖子评论表
type PostComment struct {
	ID        int64          `gorm:"primaryKey;autoIncrement:false" json:"id"`
	SchoolID  int64          `gorm:"column:school_id;not null;index" json:"school_id"`
	PostID    int64          `gorm:"column:post_id;not null;index"   json:"post_id"`
	UserID    int64          `gorm:"column:user_id;not null;index"   json:"user_id"`
	Content   string         `gorm:"size:500;not null"               json:"content"`
	ParentID  int64          `gorm:"column:parent_id;default:0"      json:"parent_id"` // 父评论ID（Phase 2 二级回复使用）
	Status    int8           `gorm:"default:1;index"                 json:"status"`    // 1=normal 2=deleted
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (PostComment) TableName() string { return "post_comments" }