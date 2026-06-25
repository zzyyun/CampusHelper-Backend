package repo

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"go_projects/praProject1/cmd/message/model"
	"go_projects/praProject1/pkg/db"
	"go_projects/praProject1/pkg/snowflake"

	"gorm.io/gorm"
)

// mustMessageDB 返回 message 服务的 *gorm.DB，若未初始化则记录 fatal。
func mustMessageDB() *gorm.DB {
	d, err := db.GetMessageDB()
	if err != nil {
		log.Fatalf("[message-repo] 未初始化 message db: %v", err)
	}
	return d
}

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("记录不存在")

// ─── CRUD ────────────────────────────────────────────────────────────────────

// Create 创建一条通知记录。
func Create(userID, schoolID int64, notifType, title, content string, fromUserID int64, refType string, refID int64) (*model.Notification, error) {
	n := &model.Notification{
		ID:         genNotificationID(),
		SchoolID:   schoolID,
		UserID:     userID,
		Type:       notifType,
		Title:      title,
		Content:    content,
		FromUserID: fromUserID,
		RefType:    refType,
		RefID:      refID,
		IsRead:     false,
	}
	if err := mustMessageDB().Create(n).Error; err != nil {
		return nil, fmt.Errorf("创建通知: %w", err)
	}
	return n, nil
}

// ─── 查询 ────────────────────────────────────────────────────────────────────

// Cursor 游标分页参数
type Cursor struct {
	ID        int64 `json:"id"`
	CreatedAt int64 `json:"created_at"`
}

// EncodeCursor 将游标编码为 Base64+JSON 字符串。
func EncodeCursor(cursor Cursor) string {
	if cursor.ID == 0 {
		return ""
	}
	data, _ := json.Marshal(cursor)
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeCursor 解码游标。
func DecodeCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("游标解码失败: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, fmt.Errorf("游标 JSON 解析失败: %w", err)
	}
	return c, nil
}

// ListByUser 查询用户的通知列表（游标分页）。
// 支持按 type 筛选。返回通知列表、是否有更多页、下一页游标。
func ListByUser(userID, schoolID int64, cursorStr string, pageSize int, typeFilter string) ([]model.Notification, bool, string, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	dbQuery := mustMessageDB().Where("user_id = ? AND school_id = ?", userID, schoolID)

	// 按类型筛选
	if typeFilter != "" {
		dbQuery = dbQuery.Where("type = ?", typeFilter)
	}

	// 游标分页：查询比游标更早（created_at 更小或相等但 id 更小）的记录
	cursor, err := DecodeCursor(cursorStr)
	if err != nil {
		return nil, false, "", fmt.Errorf("无效游标: %w", err)
	}
	if cursor.ID > 0 {
		dbQuery = dbQuery.Where("(created_at < ? OR (created_at = ? AND id < ?))",
			time.Unix(cursor.CreatedAt, 0), time.Unix(cursor.CreatedAt, 0), cursor.ID)
	}

	// 多查一条判断 has_more
	var notifications []model.Notification
	if err := dbQuery.Order("created_at DESC, id DESC").
		Limit(pageSize + 1).Find(&notifications).Error; err != nil {
		return nil, false, "", fmt.Errorf("查询通知列表: %w", err)
	}

	hasMore := len(notifications) > pageSize
	if hasMore {
		notifications = notifications[:pageSize]
	}

	// 计算下一页游标
	nextCursor := ""
	if hasMore && len(notifications) > 0 {
		last := notifications[len(notifications)-1]
		nextCursor = EncodeCursor(Cursor{ID: last.ID, CreatedAt: last.CreatedAt.Unix()})
	}

	return notifications, hasMore, nextCursor, nil
}

// UnreadCount 查询用户未读通知数。
func UnreadCount(userID, schoolID int64) (int64, error) {
	var count int64
	if err := mustMessageDB().Model(&model.Notification{}).
		Where("user_id = ? AND school_id = ? AND is_read = ?", userID, schoolID, false).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("查询未读数: %w", err)
	}
	return count, nil
}

// ─── 更新 ────────────────────────────────────────────────────────────────────

// MarkRead 标记单条通知为已读。
func MarkRead(id, userID int64) error {
	result := mustMessageDB().Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)
	if result.Error != nil {
		return fmt.Errorf("标记已读: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkAllRead 标记用户的所有通知为已读。
func MarkAllRead(userID, schoolID int64) (int64, error) {
	result := mustMessageDB().Model(&model.Notification{}).
		Where("user_id = ? AND school_id = ? AND is_read = ?", userID, schoolID, false).
		Update("is_read", true)
	if result.Error != nil {
		return 0, fmt.Errorf("全部标记已读: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ─── 删除 ────────────────────────────────────────────────────────────────────

// SoftDelete 软删除单条通知。
func SoftDelete(id, userID int64) error {
	result := mustMessageDB().Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Notification{})
	if result.Error != nil {
		return fmt.Errorf("删除通知: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CleanupBefore 物理删除已软删除且创建时间早于指定时间的通知。
// 返回删除的记录数。
func CleanupBefore(before time.Time) (int64, error) {
	result := mustMessageDB().Unscoped().
		Where("deleted_at IS NOT NULL AND created_at < ?", before).
		Delete(&model.Notification{})
	if result.Error != nil {
		return 0, fmt.Errorf("清理通知: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ─── 辅助函数 ────────────────────────────────────────────────────────────────

// genNotificationID 生成通知 ID（使用雪花算法）。
func genNotificationID() int64 {
	return snowflake.GenerateID()
}

