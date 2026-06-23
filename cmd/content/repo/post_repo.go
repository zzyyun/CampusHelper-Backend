package repo

import (
	"errors"
	"log"

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