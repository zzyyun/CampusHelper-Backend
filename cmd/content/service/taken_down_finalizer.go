// Package service - taken_down_finalizer.go 提供 24h 宽限期结束器。
//
// 用途（PRD rev2 § Story 5）：
//   - 每小时扫描 status=taken_down_pending 的帖子
//   - 检查 created_at < now-24h 且无申诉 → 转为 status=closed（最终下架）
//   - 发布 content.taken_down MQ 事件
//
// 申诉入口（v3.x 完整流程）：
//   - 本期由客服兜底，投诉字段记录到 ai_audit_logs.Detail
//   - 申诉成功：finalizer 跳过（人工已恢复为 published）
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 任务 task-045 (#98)
package service

import (
	"context"
	"log"
	"time"

	"go_projects/praProject1/cmd/content/model"
	content_repo "go_projects/praProject1/cmd/content/repo"
	"go_projects/praProject1/pkg/mq"
)

// TakenDownFinalizer 24h 宽限期结束器
type TakenDownFinalizer struct {
	mqAddr string
	stopCh chan struct{}
}

// NewTakenDownFinalizer 创建 finalizer
func NewTakenDownFinalizer(mqAddr string) *TakenDownFinalizer {
	return &TakenDownFinalizer{
		mqAddr: mqAddr,
		stopCh: make(chan struct{}),
	}
}

// Start 启动 finalizer（非阻塞）
//
// 调度策略：每 1 小时执行一次扫描
// 首次启动立即执行一次扫描
func (f *TakenDownFinalizer) Start(ctx context.Context) {
	log.Printf("[TakenDownFinalizer] Starting (interval=1h, grace=24h)")
	go f.run(ctx)
}

// Stop 停止 finalizer
func (f *TakenDownFinalizer) Stop() {
	close(f.stopCh)
}

// run 主循环
func (f *TakenDownFinalizer) run(ctx context.Context) {
	// 首次立即执行
	f.scanAndFinalize(ctx)

	// 之后每 1 小时
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			f.scanAndFinalize(ctx)
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// scanAndFinalize 扫描 taken_down_pending 帖子并执行最终下架
//
// 流程：
//   1. 查询所有 status=taken_down_pending 且 taken_down_at < now-24h 的帖子
//   2. 检查是否有申诉记录（本期由客服兜底，查询 audit_logs 标记）
//   3. 无申诉 → UpdateStatus(taken_down_pending → closed)
//   4. 发布 content.taken_down MQ 事件
//   5. 记录 ai_audit_logs (status=closed-by-grace)
func (f *TakenDownFinalizer) scanAndFinalize(ctx context.Context) {
	log.Printf("[TakenDownFinalizer] Scan started at %s", time.Now().Format(time.RFC3339))

	// 计算 24h 前时间
	deadline := time.Now().Add(-24 * time.Hour)

	// 查询待 finalizer 的帖子列表
	// 注：当前为简化实现，使用通用查询；后续 task-047 提供专用 DAO
	posts, err := scanTakenDownPendingPosts(deadline)
	if err != nil {
		log.Printf("[TakenDownFinalizer] scan failed: %v", err)
		return
	}

	log.Printf("[TakenDownFinalizer] Found %d posts past grace period", len(posts))

	for _, post := range posts {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		default:
		}

		// 检查是否有申诉记录
		if hasAppeal(post.ID) {
			log.Printf("[TakenDownFinalizer] post %d has appeal, skip (manual review)", post.ID)
			continue
		}

		// 最终下架
		if err := content_repo.UpdateStatus(post.SchoolID, post.ID, model.PostStatusTakenDownPending, model.PostStatusClosed); err != nil {
			log.Printf("[TakenDownFinalizer] update post %d status failed: %v", post.ID, err)
			continue
		}

		// 发布 content.taken_down MQ 事件（最终下架通知）
		f.publishTakenDown(ctx, post)

		log.Printf("[TakenDownFinalizer] post %d → closed (grace period expired)", post.ID)
	}

	log.Printf("[TakenDownFinalizer] Scan completed")
}

// publishTakenDown 发布 content.taken_down MQ 事件
func (f *TakenDownFinalizer) publishTakenDown(ctx context.Context, post PostInfo) {
	if notificationPublisher == nil {
		log.Printf("[TakenDownFinalizer] notification publisher not initialized, skip MQ event")
		return
	}
	event := mq.ContentEvent{
		Type:     "content.taken_down",
		PostID:   post.ID,
		SchoolID: post.SchoolID,
		UserID:   post.UserID,
		Data: map[string]string{
			"reason":      "ai_async_review_grace_expired",
			"finalized_at": time.Now().Format(time.RFC3339),
		},
	}
	if err := notificationPublisher.Publish(ctx, event); err != nil {
		log.Printf("[TakenDownFinalizer] publish taken_down failed: %v", err)
	}
}

// PostInfo finalizer 使用的简化帖子信息
type PostInfo struct {
	ID       int64
	SchoolID int64
	UserID   int64
}

// scanTakenDownPendingPosts 扫描过宽限期的 taken_down_pending 帖子
//
// 注：当前为简化实现，需要 task-047 提供专用 DAO
// 实际 SQL: SELECT id, school_id, user_id FROM posts
//   WHERE status=8 AND updated_at < ? AND deleted_at IS NULL
func scanTakenDownPendingPosts(deadline time.Time) ([]PostInfo, error) {
	log.Printf("[TakenDownFinalizer] scanTakenDownPendingPosts stub (deadline=%s)", deadline.Format(time.RFC3339))
	return nil, nil
}

// hasAppeal 检查帖子是否有申诉记录
//
// 当前为简化实现：返回 false（无申诉）
// 完整实现需查询 ai_audit_logs.Detail 或独立的 appeals 表
func hasAppeal(postID int64) bool {
	return false
}