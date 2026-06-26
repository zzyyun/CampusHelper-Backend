// Package service - ai_moderation.go 提供 Content Service 调用 ai-moderation 的封装。
//
// 设计要点：
//   - 同步调用：DFA → AI 同步（800ms 超时）→ 状态决策
//   - fallback：AI 不可用时降级到仅 DFA 模式
//   - ai_audit_logs：无论 AI 是否成功，都记录审计日志
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 任务 task-042 (#94), 后续 task-044 (#96 异步), task-045 (#98 宽限期)
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"
	content_db "go_projects/praProject1/cmd/content/database"
	"go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/pkg/aiclient"
)

// ─── 全局 AI 客户端（由 main.go InitAIClient 初始化）───────────────────────

var (
	aiClient   aiclient.ModerationClient
	aiClientMu sync.RWMutex
)

// InitAIClient 初始化 AI 客户端（在 content main.go 启动时调用）
//
// 参数：
//   - client: aiclient.NewClient 返回的客户端实例
func InitAIClient(client aiclient.ModerationClient) {
	aiClientMu.Lock()
	defer aiClientMu.Unlock()
	aiClient = client
}

// GetAIClient 获取当前 AI 客户端（供测试或调试用）
func GetAIClient() aiclient.ModerationClient {
	aiClientMu.RLock()
	defer aiClientMu.RUnlock()
	return aiClient
}

// ─── 审核决策辅助函数 ──────────────────────────────────────────────────────

// AIModerationDecision AI 审核决策结果
type AIModerationDecision struct {
	// AIStatus 调用状态（与 pb.ModerateTextResponse.Status 对应）
	AIStatus int32
	// AIResult 决策结果（PASS/REVIEW/BLOCK/DEGRADED）
	AIResult int32
	// Confidence 置信度
	Confidence float32
	// Categories 命中类别
	Categories []string
	// FallbackUsed 是否走降级
	FallbackUsed bool
	// ModelVersion 模型版本
	ModelVersion string
	// LatencyMs 调用延迟
	LatencyMs int64
}

// callAIModeration 调用 AI 审核服务（带 fallback）
//
// 参数：
//   - ctx: 调用方上下文（应包含 trace_id）
//   - text: 待审核文本
//   - postID: 帖子 ID
//
// 返回：
//   - *AIModerationDecision: 决策结果（无论 AI 是否成功）
//   - error: 严重错误（仅日志记录，不影响主流程）
//
// 行为：
//   - aiClient 为 nil → fallback（返回 DEGRADED + PASS）
//   - aiClient 调用失败 → fallback
//   - aiClient 调用超时 → fallback
//   - aiClient 熔断 → fallback
//   - 成功 → 返回 AI 决策
func callAIModeration(ctx context.Context, text string, postID int64) *AIModerationDecision {
	start := time.Now()

	client := GetAIClient()
	if client == nil {
		// AI 客户端未初始化 → fallback
		return &AIModerationDecision{
			AIStatus:     int32(ai_moderation_pb.ModerateTextResponse_DEGRADED),
			AIResult:     int32(ai_moderation_pb.ModerateTextResponse_PASS),
			FallbackUsed: true,
			ModelVersion: "uninitialized",
			LatencyMs:    time.Since(start).Milliseconds(),
		}
	}

	// 调用 AI（800ms 超时由 aiclient 内部处理）
	resp, err := client.ModerateText(ctx, text, postID)
	if err != nil {
		// 调用失败（连接/超时/熔断）→ fallback
		log.Printf("[content-service] AI moderation failed (post_id=%d): %v, fallback to DFA-only",
			postID, err)
		return &AIModerationDecision{
			AIStatus:     int32(ai_moderation_pb.ModerateTextResponse_DEGRADED),
			AIResult:     int32(ai_moderation_pb.ModerateTextResponse_PASS),
			FallbackUsed: true,
			ModelVersion: "unavailable",
			LatencyMs:    time.Since(start).Milliseconds(),
		}
	}

	return &AIModerationDecision{
		AIStatus:     int32(resp.Status),
		AIResult:     int32(resp.Result),
		Confidence:   resp.Confidence,
		Categories:   resp.Categories,
		FallbackUsed: resp.FallbackUsed,
		ModelVersion: resp.ModelVersion,
		LatencyMs:    resp.LatencyMs,
	}
}

// decidePostStatus 根据 AI 决策返回帖子最终状态
//
// 映射规则：
//   - AI PASS (含 DEGRADED fallback) → published
//   - AI REVIEW → pending_review
//   - AI BLOCK → rejected
func decidePostStatus(decision *AIModerationDecision) (model.PostStatus, bool) {
	switch ai_moderation_pb.ModerateTextResponse_Result(decision.AIResult) {
	case ai_moderation_pb.ModerateTextResponse_PASS:
		return model.PostStatusPublished, true
	case ai_moderation_pb.ModerateTextResponse_REVIEW:
		return model.PostStatusPending, false // 需人工复审
	case ai_moderation_pb.ModerateTextResponse_BLOCK:
		return model.PostStatusRejected, false
	default:
		// 未知值 → REVIEW（安全兜底，与 PRD 一致）
		return model.PostStatusPending, false
	}
}

// recordAIAuditLog 记录 AI 审计日志（写入失败仅 WARN，不阻塞主流程）
func recordAIAuditLog(postID int64, decision *AIModerationDecision, text string, traceID string) {
	auditLog := &model.AIAuditLog{
		ID:           nextAIAuditLogID(),
		PostID:       postID,
		ContentHash:  sha256Hex(text),
		AIStatus:     model.AIStatus(decision.AIStatus),
		AIResult:     model.AIResult(decision.AIResult),
		AIConfidence: decision.Confidence,
		LatencyMs:    decision.LatencyMs,
		ModelVersion: decision.ModelVersion,
		FallbackUsed: decision.FallbackUsed,
		TraceID:      traceID,
	}
	if len(decision.Categories) > 0 {
		if b, err := json.Marshal(decision.Categories); err == nil {
			auditLog.AICategories = string(b)
		}
	}
	if err := content_db.CreateAIAuditLog(auditLog); err != nil {
		// 不阻塞发帖，仅 WARN 日志
		log.Printf("[content-service] ai_audit_log write failed (post_id=%d): %v", postID, err)
	}
}

// ─── 雪花 ID 生成（与帖子 ID 共用 worker）──────────────────────────────────

var aiAuditLogIDCounter int64

// nextAIAuditLogID 生成 ai_audit_logs 表的雪花 ID
//
// 注：与帖子 ID 共用 worker（同毫秒内唯一），实际生产应使用独立 worker
// 当前为简化实现，使用时间戳+自增
func nextAIAuditLogID() int64 {
	now := time.Now().UnixMilli()
	aiAuditLogIDCounter++
	return now*1000 + aiAuditLogIDCounter
}

// ─── 哈希工具 ──────────────────────────────────────────────────────────────

// sha256Hex 计算文本 SHA256（不存原始文本，隐私保护）
func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// ─── trace_id 提取 ─────────────────────────────────────────────────────────

// extractTraceID 从 ctx 提取 trace_id（与 pkg/contextx 兼容）
func extractTraceID(ctx context.Context) string {
	if v := ctx.Value("trace_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	// fallback: 从 gRPC metadata 提取
	if md, ok := ctx.Value("x-request-id").(string); ok {
		return md
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}