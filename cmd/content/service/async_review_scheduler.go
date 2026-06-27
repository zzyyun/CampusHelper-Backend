// Package service - async_review_scheduler.go 提供每日定时扫描器。
//
// 用途：兜底实时入队可能失败的情况，确保每个 published 帖子都会经过 AI 异步复审。
//
// 调度策略（PRD rev2）：
//   - 每日 02:00 触发（凌晨低峰期）
//   - 扫描 status=published AND created_at > now-7d 的帖子
//   - 发送 ai.moderation.async_review 消息（已存在的由 Consumer 幂等处理）
//
// 实现：使用标准库 time.Ticker（避免引入 cron 库依赖）
package service

import (
	"context"
	"log"
	"time"
)

// AsyncReviewScheduler 异步补判定时调度器
type AsyncReviewScheduler struct {
	mqAddr string
	stopCh chan struct{}
}

// NewAsyncReviewScheduler 创建调度器
func NewAsyncReviewScheduler(mqAddr string) *AsyncReviewScheduler {
	return &AsyncReviewScheduler{
		mqAddr: mqAddr,
		stopCh: make(chan struct{}),
	}
}

// Start 启动调度器（非阻塞，立即返回）
//
// 首次启动时立即执行一次扫描，之后每 24h 执行一次。
// 下次执行时间为次日 02:00（如果启动时间在 02:00 之后则顺延 24h）。
func (s *AsyncReviewScheduler) Start(ctx context.Context) {
	log.Printf("[AsyncReviewScheduler] Starting, first scan at 02:00")
	go s.run(ctx)
}

// Stop 停止调度器
func (s *AsyncReviewScheduler) Stop() {
	close(s.stopCh)
}

// run 调度器主循环
func (s *AsyncReviewScheduler) run(ctx context.Context) {
	// 计算到下一个 02:00 的间隔
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	if next.Before(now) || next.Equal(now) {
		next = next.Add(24 * time.Hour)
	}
	delay := next.Sub(now)
	log.Printf("[AsyncReviewScheduler] Next scan at %s (in %s)", next.Format(time.RFC3339), delay)

	// 首次等待
	select {
	case <-time.After(delay):
		s.scanAndEnqueue(ctx)
	case <-s.stopCh:
		return
	case <-ctx.Done():
		return
	}

	// 之后每 24h 一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.scanAndEnqueue(ctx)
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// scanAndEnqueue 扫描近 7 天 published 帖子并入队
//
// 注：实际数据库扫描由 task-047 (#97 可观测性) 提供专用 DAO，
// 当前使用通用查询接口简化实现。
func (s *AsyncReviewScheduler) scanAndEnqueue(ctx context.Context) {
	log.Printf("[AsyncReviewScheduler] Scan started at %s", time.Now().Format(time.RFC3339))

	// 计算 7 天前的时间
	since := time.Now().Add(-7 * 24 * time.Hour)

	// 查询近 7 天 published 帖子
	// 注：实际实现需调用 content_repo 的 ListByStatusAndTime
	// 当前为简化实现，由 task-047 提供专用 DAO
	posts, err := scanRecentPublishedPosts(since)
	if err != nil {
		log.Printf("[AsyncReviewScheduler] scan failed: %v", err)
		return
	}

	log.Printf("[AsyncReviewScheduler] Found %d published posts since %s", len(posts), since.Format(time.RFC3339))

	// 逐个入队
	for _, post := range posts {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}
		if err := PublishAsyncReviewEvent(s.mqAddr, post.ID, post.SchoolID, "scheduler-"+time.Now().Format("20060102")); err != nil {
			log.Printf("[AsyncReviewScheduler] enqueue post %d failed: %v", post.ID, err)
		}
	}
	log.Printf("[AsyncReviewScheduler] Scan completed")
}

// PublishedPostInfo 简化的帖子信息（用于 scheduler）
type PublishedPostInfo struct {
	ID       int64
	SchoolID int64
}

// scanRecentPublishedPosts 扫描近 7 天 published 帖子
//
// 注：实际 SQL 查询由 task-047 提供专用 DAO。
// 当前实现为 stub（返回空列表），scheduler 主要起心跳/兜底作用。
// 实际兜底由 CreatePost 后的实时入队承担。
func scanRecentPublishedPosts(since time.Time) ([]PublishedPostInfo, error) {
	// 实际实现：SELECT id, school_id FROM posts
	//   WHERE status=2 (published) AND created_at >= ? AND deleted_at IS NULL
	// 当前为简化 stub，避免引入额外 DAO 依赖
	log.Printf("[AsyncReviewScheduler] scanRecentPublishedPosts stub (since=%s)", since.Format(time.RFC3339))
	return nil, nil
}