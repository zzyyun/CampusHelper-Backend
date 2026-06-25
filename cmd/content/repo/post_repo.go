package repo

import (
	"errors"
	"log"
	"time"

	content_db "go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/pkg/db"

	"gorm.io/gorm"
)

// mustContentDB 返回 content 服务的 *gorm.DB。
// main.go 必须先执行 db.InitContentDB()，否则本函数会记录 fatal。
func mustContentDB() *gorm.DB {
	d, err := db.GetContentDB()
	if err != nil {
		log.Fatalf("[content-repo] 未初始化 content db: %v", err)
	}
	return d
}

// ─── 错误定义 ────────────────────────────────────────────────────────────────

var (
	ErrNotFound          = errors.New("post: 记录不存在")
	ErrForbidden         = errors.New("post: 无权操作")
	ErrInvalidTransition = errors.New("post: 非法的状态转移")
)

// ─── Post CRUD ──────────────────────────────────────────────────────────────

// Create 创建帖子
func Create(p *content_db.Post) error {
	return mustContentDB().Create(p).Error
}

// GetByID 按 ID 查询（带 school 隔离）
func GetByID(schoolID, id int64) (*content_db.Post, error) {
	var p content_db.Post
	err := mustContentDB().Scopes(db.SchoolScope(schoolID)).First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateOwned 由作者本人更新帖子（前置权限校验）
func UpdateOwned(schoolID, userID, id int64, fields map[string]interface{}) error {
	res := mustContentDB().
		Model(&content_db.Post{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// 要么记录不存在，要么不是作者
		return ErrForbidden
	}
	return nil
}

// DeleteOwned 由作者本人软删除帖子
func DeleteOwned(schoolID, userID, id int64) error {
	res := mustContentDB().
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&content_db.Post{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrForbidden
	}
	return nil
}

// ListByCursor 游标分页查询（按 ID 倒序）
// cursor 为空表示从最新开始；status 过滤（如已发布）
func ListByCursor(schoolID int64, postType, status int8, cursor int64, pageSize int) ([]content_db.Post, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := mustContentDB().
		Model(&content_db.Post{}).
		Scopes(db.SchoolScope(schoolID))
	if postType > 0 {
		q = q.Where("type = ?", postType)
	}
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var posts []content_db.Post
	if err := q.Order("id DESC").Limit(pageSize + 1).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	var nextCursor int64
	if len(posts) > pageSize {
		nextCursor = posts[pageSize-1].ID
		posts = posts[:pageSize]
	}
	return posts, nextCursor, nil
}

// UpdateStatus 更新状态（带状态机校验 + school 隔离）
func UpdateStatus(schoolID, id int64, from, to content_db.PostStatus) error {
	// 状态机校验在调用方完成，这里只做受影响的行更新
	res := mustContentDB().
		Model(&content_db.Post{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ? AND status = ?", id, from).
		Update("status", to)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvalidTransition
	}
	return nil
}

// UpdateReview 审核操作：原子更新状态 + 审核员 + 审核时间 + 拒绝原因。
// 若 from 状态不匹配（已被其他操作修改），返回 ErrInvalidTransition。
func UpdateReview(schoolID, id int64, from, to content_db.PostStatus, reviewerID int64, reason string) error {
	now := time.Now()
	fields := map[string]interface{}{
		"status":      to,
		"reviewer_id": reviewerID,
		"reviewed_at": now,
	}
	if reason != "" {
		fields["reject_reason"] = reason
	}
	res := mustContentDB().
		Model(&content_db.Post{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ? AND status = ?", id, from).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvalidTransition
	}
	return nil
}

// IncLikesCount / DecLikesCount 原子增减点赞计数
// 失败不影响主流程，仅记录日志
func IncLikesCount(id int64) {
	if err := mustContentDB().Model(&content_db.Post{}).
		Where("id = ?", id).
		UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
		log.Printf("[content-repo] inc likes_count id=%d: %v", id, err)
	}
}

func DecLikesCount(id int64) {
	if err := mustContentDB().Model(&content_db.Post{}).
		Where("id = ? AND likes_count > 0", id).
		UpdateColumn("likes_count", gorm.Expr("likes_count - 1")).Error; err != nil {
		log.Printf("[content-repo] dec likes_count id=%d: %v", id, err)
	}
}

// ─── Post Like ──────────────────────────────────────────────────────────────

// AddLike 添加点赞（同一用户重复点赞会被唯一索引拦截，错误由调用方处理）
func AddLike(schoolID, postID, userID int64) (bool, error) {
	like := &content_db.PostLike{
		ID:       0, // 由 snowflake 生成
		SchoolID: schoolID,
		PostID:   postID,
		UserID:   userID,
	}
	// 使用 ON DUPLICATE KEY UPDATE 兼容幂等
	res := mustContentDB().
		Where("post_id = ? AND user_id = ?", postID, userID).
		FirstOrCreate(like)
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		// 记录已存在，幂等返回 false（未新增）
		return false, nil
	}
	return true, nil
}

// RemoveLike 取消点赞
func RemoveLike(schoolID, postID, userID int64) (bool, error) {
	res := mustContentDB().
		Where("school_id = ? AND post_id = ? AND user_id = ?", schoolID, postID, userID).
		Delete(&content_db.PostLike{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// HasLiked 判断用户是否已点赞
func HasLiked(schoolID, postID, userID int64) (bool, error) {
	var n int64
	err := mustContentDB().Model(&content_db.PostLike{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Count(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ─── Post Comment ───────────────────────────────────────────────────────────

// CreateComment 创建评论并原子递增帖子评论计数。
func CreateComment(comment *content_db.PostComment) error {
	gdb := mustContentDB()
	return gdb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		// 原子递增评论计数
		return tx.Model(&content_db.Post{}).
			Where("id = ?", comment.PostID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
	})
}

// DeleteOwnedComment 软删除评论（仅作者本人，校验 school_id + user_id）。
// 返回 true 表示删除了正常评论，false 表示已删除或不存在。
func DeleteOwnedComment(schoolID, commentID, userID int64) (bool, error) {
	gdb := mustContentDB()
	var deleted bool
	err := gdb.Transaction(func(tx *gorm.DB) error {
		// 查找正常状态的评论（带 school 隔离）
		var c content_db.PostComment
		res := tx.Scopes(db.SchoolScope(schoolID)).
			Where("id = ? AND user_id = ? AND status = 1", commentID, userID).
			First(&c)
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return ErrForbidden
		}
		if res.Error != nil {
			return res.Error
		}

		// 软删除 + 标记 status=2（已删除）
		if err := tx.Model(&c).Updates(map[string]interface{}{
			"status": int8(2),
		}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&c).Error; err != nil {
			return err
		}

		// 原子递减（最小为 0）
		if err := tx.Model(&content_db.Post{}).
			Where("id = ? AND comment_count > 0", c.PostID).
			UpdateColumn("comment_count", gorm.Expr("comment_count - 1")).Error; err != nil {
			return err
		}
		deleted = true
		return nil
	})
	return deleted, err
}

// GetCommentByID 根据 comment_id 查询单条评论。
// 使用 SchoolScope 强制 school_id 隔离，避免跨学校越权访问。
func GetCommentByID(schoolID, commentID int64) (*content_db.PostComment, error) {
	var comment content_db.PostComment
	err := mustContentDB().
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ?", commentID).
		First(&comment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &comment, nil
}

// ListComments 游标分页查询帖子评论列表。
func ListComments(schoolID, postID int64, cursor int64, pageSize int) ([]content_db.PostComment, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	q := mustContentDB().
		Model(&content_db.PostComment{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("post_id = ? AND status = 1 AND parent_id = 0", postID) // 一级评论 + 正常状态

	if cursor > 0 {
		q = q.Where("id > ?", cursor)
	}

	var comments []content_db.PostComment
	if err := q.Order("id ASC").Limit(pageSize + 1).Find(&comments).Error; err != nil {
		return nil, 0, err
	}

	var nextCursor int64
	if len(comments) > pageSize {
		nextCursor = comments[pageSize-1].ID
		comments = comments[:pageSize]
	}
	return comments, nextCursor, nil
}