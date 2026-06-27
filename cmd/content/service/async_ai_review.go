// Package service - async_ai_review.go 提供 AI 异步补判 Consumer + 定时调度器。
//
// 流程（PRD rev2 § Feature 3）：
//   1. 帖子 published 后实时入队 ai.moderation.async_review
//   2. AsyncAIReviewConsumer 订阅该队列，5-10 goroutine 并发消费
//   3. 对已 published 帖子重新调用 AI
//      - BLOCK → status=taken_down_pending（24h 宽限期）+ MQ content.taken_down_pending
//      - PASS/REVIEW → 不动状态（保守）
//   4. AsyncReviewScheduler 每日 02:00 兜底扫描（防实时入队遗漏）
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 任务 task-044 (#96), 后续 task-045 (#98 宽限期 finalizer)
package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"
	"go_projects/praProject1/cmd/content/database"
	"go_projects/praProject1/cmd/content/model"
	content_repo "go_projects/praProject1/cmd/content/repo"
	"go_projects/praProject1/pkg/aiclient"
	"go_projects/praProject1/pkg/mq"
)

// jsonMarshal / jsonUnmarshal 包装便于测试和未来替换
var (
	jsonMarshal   = json.Marshal
	jsonUnmarshal = json.Unmarshal
)

// AIAsyncReviewEvent 异步补判事件（队列消息体）
type AIAsyncReviewEvent struct {
	Type     string `json:"type"`     // "ai.async.review"
	PostID   int64  `json:"post_id"`  // 待复审帖子 ID
	SchoolID int64  `json:"school_id"`
	TraceID  string `json:"trace_id"` // 关联 Jaeger
}

// AsyncAIReviewConsumer AI 异步补判消费者
type AsyncAIReviewConsumer struct {
	consumer *mq.Consumer
	aiClient aiclient.ModerationClient
	addr     string

	// 并发控制
	concurrency int
	sem         chan struct{}
	wg          sync.WaitGroup
}

// NewAsyncAIReviewConsumer 创建异步 AI 复审消费者
//
// 参数：
//   - mqAddr: amqp://user:pass@host:port/
//   - aiClient: aiclient.ModerationClient 实例
//   - concurrency: 并发 goroutine 数（默认 5）
func NewAsyncAIReviewConsumer(mqAddr string, aiClient aiclient.ModerationClient, concurrency int) *AsyncAIReviewConsumer {
	if concurrency <= 0 {
		concurrency = 5
	}
	c := mq.NewConsumer(mqAddr, "ai.moderation.async_review")
	consumer := &AsyncAIReviewConsumer{
		consumer:    c,
		aiClient:    aiClient,
		addr:        mqAddr,
		concurrency: concurrency,
		sem:         make(chan struct{}, concurrency),
	}
	// 注册处理器（自定义消息类型）
	consumer.consumer.RegisterHandler("ai.async.review", consumer.handleAsyncReview)
	return consumer
}

// Start 启动消费者（阻塞调用）
func (c *AsyncAIReviewConsumer) Start(ctx context.Context) error {
	log.Printf("[AsyncAIReview] Starting consumer (concurrency=%d)", c.concurrency)
	return c.consumer.Start(ctx)
}

// Stop 停止消费者
func (c *AsyncAIReviewConsumer) Stop() {
	c.consumer.Stop()
	c.wg.Wait()
	log.Printf("[AsyncAIReview] Consumer stopped")
}

// handleAsyncReview 处理异步复审消息
func (c *AsyncAIReviewConsumer) handleAsyncReview(ctx context.Context, event *mq.ContentEvent) error {
	if event.PostID <= 0 {
		log.Printf("[AsyncAIReview] invalid post_id in event")
		return nil
	}

	// 并发控制
	c.sem <- struct{}{}
	c.wg.Add(1)
	defer func() {
		<-c.sem
		c.wg.Done()
	}()

	return c.processAsyncReview(ctx, event.PostID, event.SchoolID, event.UserID)
}

// processAsyncReview 单个帖子的异步复审流程
func (c *AsyncAIReviewConsumer) processAsyncReview(ctx context.Context, postID, schoolID, userID int64) error {
	// 查询帖子
	post, err := content_repo.GetByID(schoolID, postID)
	if err != nil {
		log.Printf("[AsyncAIReview] post %d not found: %v", postID, err)
		return nil // 帖子不存在，丢弃（不可恢复）
	}

	// 仅处理 published 状态（已下架的跳过）
	if post.Status != model.PostStatusPublished {
		log.Printf("[AsyncAIReview] post %d status=%d, skip", postID, post.Status)
		return nil
	}

	// 调用 AI 重新审核
	if c.aiClient == nil {
		log.Printf("[AsyncAIReview] AI client not initialized, skip post %d", postID)
		return nil
	}

	resp, err := c.aiClient.ModerateText(ctx, post.Title+"\n"+post.Content, postID)
	if err != nil {
		log.Printf("[AsyncAIReview] AI call failed for post %d: %v (will retry via DLQ)", postID, err)
		return err // 返回 error → 消息 Nack 重新入队
	}

	// 记录 ai_audit_logs (status=async)
	auditLog := &model.AIAuditLog{
		ID:           nextAIAuditLogID(),
		PostID:       postID,
		ContentHash:  sha256Hex(post.Title + "\n" + post.Content),
		AIStatus:     model.AIStatus(int32(ai_moderation_pb.ModerateTextResponse_ASYNC)),
		AIResult:     model.AIResult(int32(resp.Result)),
		AIConfidence: resp.Confidence,
		LatencyMs:    resp.LatencyMs,
		ModelVersion: resp.ModelVersion,
		FallbackUsed: resp.FallbackUsed,
		TraceID:      "async-" + time.Now().Format("20060102150405"),
	}
	if len(resp.Categories) > 0 {
		if b, err := jsonMarshal(resp.Categories); err == nil {
			auditLog.AICategories = string(b)
		}
	}
	if err := database.CreateAIAuditLog(auditLog); err != nil {
		log.Printf("[AsyncAIReview] audit log write failed (post_id=%d): %v", postID, err)
	}

	// 根据 AI 决策执行操作
	switch resp.Result {
	case ai_moderation_pb.ModerateTextResponse_BLOCK:
		// 异步补判违规 → taken_down_pending (24h 宽限期)
		// 实际状态变更由 task-045 (#98) 的 TakenDownFinalizer 完成终态变更
		if err := content_repo.UpdateStatus(schoolID, postID, model.PostStatusPublished, model.PostStatusTakenDownPending); err != nil {
			log.Printf("[AsyncAIReview] update post status failed: %v", err)
			return err
		}
		// 发布 MQ 事件通知发帖者"24h 内可申请复审"
		publishTakenDownPending(ctx, postID, schoolID, userID, resp.Categories)
		log.Printf("[AsyncAIReview] post %d → taken_down_pending (AI BLOCK, conf=%.2f)",
			postID, resp.Confidence)

	case ai_moderation_pb.ModerateTextResponse_PASS:
		log.Printf("[AsyncAIReview] post %d AI re-judged PASS, no action", postID)

	case ai_moderation_pb.ModerateTextResponse_REVIEW:
		// 异步补判 REVIEW → 不动状态（保守策略）
		log.Printf("[AsyncAIReview] post %d AI re-judged REVIEW, no action (keep published)", postID)

	default:
		log.Printf("[AsyncAIReview] post %d unknown AI result: %v", postID, resp.Result)
	}

	return nil
}

// ─── MQ 事件发布 ────────────────────────────────────────────────────────────

// publishTakenDownPending 发布 content.taken_down_pending MQ 事件
func publishTakenDownPending(ctx context.Context, postID, schoolID, userID int64, categories []string) {
	if notificationPublisher == nil {
		log.Printf("[AsyncAIReview] notification publisher not initialized, skip MQ event")
		return
	}
	event := mq.ContentEvent{
		Type:     "content.taken_down_pending",
		PostID:   postID,
		SchoolID: schoolID,
		UserID:   userID,
		Data: map[string]string{
			"categories":          joinStrings(categories, ","),
			"grace_period_hours":  "24",
			"deadline":            time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		},
	}
	if err := notificationPublisher.Publish(ctx, event); err != nil {
		log.Printf("[AsyncAIReview] publish taken_down_pending failed: %v", err)
	}
}

// joinStrings 简单字符串拼接（避免引入 strings.Join 的循环引用）
func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for _, s := range items[1:] {
		result += sep + s
	}
	return result
}

// ─── 实时入队辅助函数 ──────────────────────────────────────────────────────

// PublishAsyncReviewEvent 发布异步补判消息到队列
//
// 调用方：Content Service.CreatePost 成功后，异步入队（避免遗漏）
func PublishAsyncReviewEvent(mqAddr string, postID, schoolID int64, traceID string) error {
	// 异步发送（不阻塞发帖主流程）
	go func() {
		event := mq.ContentEvent{
			Type:     "ai.async.review",
			PostID:   postID,
			SchoolID: schoolID,
			UserID:   0, // userID 不在此场景使用
			Data: map[string]string{
				"trace_id": traceID,
			},
		}
		pub := mq.NewPublisher(mqAddr, "ai.moderation.async_review")
		if err := pub.Publish(context.Background(), event); err != nil {
			log.Printf("[AsyncAIReview] publish async review event failed (post_id=%d): %v", postID, err)
		}
	}()
	return nil
}