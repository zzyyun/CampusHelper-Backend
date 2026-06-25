package service

import (
	"context"
	"fmt"
	"log"

	"go_projects/praProject1/cmd/message/model"
	"go_projects/praProject1/cmd/message/repo"
	"go_projects/praProject1/pkg/mq"
)

// ─── 事件处理器 ─────────────────────────────────────────────────────────────
//
// 所有处理器遵循同一模式：
//  1. 从事件解析必要字段
//  2. 格式化通知标题（含快照）
//  3. 调用 repo.Create 持久化通知
//  4. 返回 nil（Ack）或 error（Nack+requeue）
//
// 注意：当前版本使用事件中的 raw user_id/post_id 构造标题。
// 下期接入 User Service（获取昵称）和 Content Service（获取帖子标题）后，
// 标题将包含真实的用户名和帖子名快照。

// ─── 标题格式化（导出，供测试） ─────────────────────────────────────────────

// FormatReviewTitle 格式化审核结果通知标题。
// eventType: content.published 或 content.review_result
// reason: 拒绝原因（review_result 时使用）
func FormatReviewTitle(eventType string, reason string) (title string, notifType string) {
	if eventType == mq.EventContentRejected {
		if reason == "" {
			reason = "未提供原因"
		}
		return fmt.Sprintf("你的帖子审核未通过，原因: %s", reason), string(model.NotifReviewResult)
	}
	return "你的帖子已通过审核", string(model.NotifPublished)
}

// FormatTakenDownTitle 格式化违规下架通知标题。
func FormatTakenDownTitle(reason string) string {
	if reason == "" {
		reason = "未提供原因"
	}
	return fmt.Sprintf("你的帖子因违规已下架，原因: %s", reason)
}

// FormatRepliedTitle 格式化评论回复通知标题。
func FormatRepliedTitle(contentPreview string) string {
	if contentPreview == "" {
		contentPreview = "回复了你的评论"
	}
	return fmt.Sprintf("有人回复了你的评论: %s", contentPreview)
}

// ─── 事件处理器 ─────────────────────────────────────────────────────────────

// HandleLiked 处理 content.liked 事件：有人点赞帖子 → 通知帖子作者。
func HandleLiked(ctx context.Context, event *mq.ContentEvent) error {
	// event.UserID = 点赞者
	// event.PostID = 被点赞的帖子
	// 需要知道帖子作者是谁 → 当前无法从事件中获取
	// 此处需要从 Content Service 查询帖子作者 user_id
	// 本期简化：仅记录日志，标记为待增强
	log.Printf("[message-service] 收到点赞事件: post=%d user=%d (待查询帖子作者)", event.PostID, event.UserID)
	return nil // Ack，不阻塞消费
}

// HandleReviewResult 处理 content.published / content.review_result 事件。
// 审核结果通知：审核通过或拒绝 → 通知发帖人。
func HandleReviewResult(ctx context.Context, event *mq.ContentEvent) error {
	title, notifType := FormatReviewTitle(event.Type, event.Data["reason"])

	_, err := repo.Create(
		event.UserID,     // 接收者 = 帖子作者
		event.SchoolID,
		notifType,
		title,
		"",
		0,                // from_user_id = 0 (系统通知)
		"post",
		event.PostID,
	)
	if err != nil {
		return fmt.Errorf("创建审核通知: %w", err)
	}
	return nil
}

// HandleTakenDown 处理 content.taken_down 事件：帖子违规下架 → 通知发帖人。
func HandleTakenDown(ctx context.Context, event *mq.ContentEvent) error {
	title := FormatTakenDownTitle(event.Data["reason"])

	_, err := repo.Create(
		event.UserID,
		event.SchoolID,
		string(model.NotifTakenDown),
		title,
		"",
		0,
		"post",
		event.PostID,
	)
	if err != nil {
		return fmt.Errorf("创建下架通知: %w", err)
	}
	return nil
}

// HandleReplied 处理 content.replied 事件：有人回复评论 → 通知父评论作者。
func HandleReplied(ctx context.Context, event *mq.ContentEvent) error {
	parentUserIDStr := event.Data["parent_comment_user_id"]
	var parentUserID int64
	if _, err := fmt.Sscanf(parentUserIDStr, "%d", &parentUserID); err != nil || parentUserID == 0 {
		log.Printf("[message-service] replied 事件缺少 parent_comment_user_id, 跳过")
		return nil
	}

	title := FormatRepliedTitle(event.Data["content_preview"])

	_, err := repo.Create(
		parentUserID,
		event.SchoolID,
		string(model.NotifReplied),
		title,
		"",
		event.UserID, // from_user_id = 回复者
		"post",
		event.PostID,
	)
	if err != nil {
		return fmt.Errorf("创建回复通知: %w", err)
	}
	return nil
}