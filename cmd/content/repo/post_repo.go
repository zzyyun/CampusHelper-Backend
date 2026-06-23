package repo

import (
	"context"
	"errors"

	"go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/pkg/db"

	"gorm.io/gorm"
)

// 公共错误定义。
var (
	ErrPostNotFound      = errors.New("帖子不存在")
	ErrInvalidStatusFlow = errors.New("非法的状态流转")
)

// PostRepo 提供帖子的数据访问能力。
// 所有查询方法强制应用 SchoolScope 多租户隔离。
type PostRepo struct {
	db *gorm.DB
}

// NewPostRepo 构造 PostRepo。
func NewPostRepo(g *gorm.DB) *PostRepo {
	return &PostRepo{db: g}
}

// CreatePost 创建帖子。
// 入参 post 必须已设置 SchoolID/UserID/Type/Title/Content/Status。
// Status 默认为 Pending（由调用方决定）。
func (r *PostRepo) CreatePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

// GetPost 根据 ID 和 schoolID 获取帖子（强制多租户隔离）。
// 跨学校查询（schoolID 错误）会返回 ErrPostNotFound 而非错误堆栈。
func (r *PostRepo) GetPost(ctx context.Context, schoolID, postID int64) (*model.Post, error) {
	var post model.Post
	err := r.db.WithContext(ctx).
		Scopes(db.SchoolScope(schoolID)).
		First(&post, postID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return &post, nil
}

// UpdatePost 更新帖子基础字段。
// 调用方需保证传入的 post 是已存在的实体。
func (r *PostRepo) UpdatePost(ctx context.Context, schoolID int64, post *model.Post) error {
	// 通过 WHERE school_id 限定，避免跨校更新
	res := r.db.WithContext(ctx).
		Scopes(db.SchoolScope(schoolID)).
		Save(post)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPostNotFound
	}
	return nil
}

// DeletePost 软删除帖子。
func (r *PostRepo) DeletePost(ctx context.Context, schoolID, postID int64) error {
	res := r.db.WithContext(ctx).
		Scopes(db.SchoolScope(schoolID)).
		Delete(&model.Post{}, postID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPostNotFound
	}
	return nil
}

// UpdateStatus 仅更新帖子状态（用于审核等场景）。
// 自动校验状态流转合法性，非法流转返回 ErrInvalidStatusFlow。
func (r *PostRepo) UpdateStatus(ctx context.Context, schoolID, postID int64, newStatus model.PostStatus) error {
	// 先查询当前状态
	post, err := r.GetPost(ctx, schoolID, postID)
	if err != nil {
		return err
	}
	// 校验状态流转
	if !model.CanTransitionTo(post.Status, newStatus) {
		return ErrInvalidStatusFlow
	}
	// 更新状态
	res := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ?", postID).
		Update("status", newStatus)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPostNotFound
	}
	return nil
}

// IncrLikesCount 原子递增 likes_count（Redis 刷回 MySQL 时使用）。
func (r *PostRepo) IncrLikesCount(ctx context.Context, schoolID, postID int64, delta int32) error {
	res := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ?", postID).
		Update("likes_count", gorm.Expr("likes_count + ?", delta))
	return res.Error
}

// IncrCommentCount 原子递增 comment_count。
func (r *PostRepo) IncrCommentCount(ctx context.Context, schoolID, postID int64, delta int32) error {
	res := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Scopes(db.SchoolScope(schoolID)).
		Where("id = ?", postID).
		Update("comment_count", gorm.Expr("comment_count + ?", delta))
	return res.Error
}