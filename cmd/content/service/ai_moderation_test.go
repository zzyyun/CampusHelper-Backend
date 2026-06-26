package service

import (
	"context"
	"testing"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"
	"go_projects/praProject1/cmd/content/model"
)

// ─── decidePostStatus 测试 ─────────────────────────────────────────────────

func TestDecidePostStatus_Pass(t *testing.T) {
	d := &AIModerationDecision{AIResult: int32(ai_moderation_pb.ModerateTextResponse_PASS)}
	status, autoPublish := decidePostStatus(d)
	if status != model.PostStatusPublished {
		t.Errorf("expected Published, got %d", status)
	}
	if !autoPublish {
		t.Error("PASS should auto-publish")
	}
}

func TestDecidePostStatus_Review(t *testing.T) {
	d := &AIModerationDecision{AIResult: int32(ai_moderation_pb.ModerateTextResponse_REVIEW)}
	status, autoPublish := decidePostStatus(d)
	if status != model.PostStatusPending {
		t.Errorf("expected Pending, got %d", status)
	}
	if autoPublish {
		t.Error("REVIEW should NOT auto-publish")
	}
}

func TestDecidePostStatus_Block(t *testing.T) {
	d := &AIModerationDecision{AIResult: int32(ai_moderation_pb.ModerateTextResponse_BLOCK)}
	status, autoPublish := decidePostStatus(d)
	if status != model.PostStatusRejected {
		t.Errorf("expected Rejected, got %d", status)
	}
	if autoPublish {
		t.Error("BLOCK should NOT auto-publish")
	}
}

func TestDecidePostStatus_Degraded(t *testing.T) {
	// DEGRADED fallback → 视为 PASS（已通过 AI Result 枚举体现）
	d := &AIModerationDecision{
		AIResult:     int32(ai_moderation_pb.ModerateTextResponse_PASS),
		FallbackUsed: true,
	}
	status, autoPublish := decidePostStatus(d)
	if status != model.PostStatusPublished {
		t.Errorf("degraded fallback should be Published, got %d", status)
	}
	if !autoPublish {
		t.Error("degraded fallback should auto-publish")
	}
}

func TestDecidePostStatus_Unknown(t *testing.T) {
	d := &AIModerationDecision{AIResult: 99}
	status, autoPublish := decidePostStatus(d)
	// 未知值 → REVIEW（安全兜底）
	if status != model.PostStatusPending {
		t.Errorf("unknown should fallback to Pending, got %d", status)
	}
	if autoPublish {
		t.Error("unknown should NOT auto-publish")
	}
}

// ─── callAIModeration fallback 测试 ────────────────────────────────────────

func TestCallAIModeration_NilClient(t *testing.T) {
	// 临时清空 client（测试 isolation）
	originalClient := GetAIClient()
	defer InitAIClient(originalClient)
	InitAIClient(nil)

	d := callAIModeration(context.Background(), "test", 123)
	if !d.FallbackUsed {
		t.Error("nil client should fallback")
	}
	if d.AIStatus != int32(ai_moderation_pb.ModerateTextResponse_DEGRADED) {
		t.Errorf("expected DEGRADED, got %d", d.AIStatus)
	}
	if d.AIResult != int32(ai_moderation_pb.ModerateTextResponse_PASS) {
		t.Errorf("expected PASS, got %d", d.AIResult)
	}
}

// ─── sha256Hex 测试 ────────────────────────────────────────────────────────

func TestSha256Hex(t *testing.T) {
	hash1 := sha256Hex("hello world")
	hash2 := sha256Hex("hello world")
	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
	if len(hash1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("SHA256 hex should be 64 chars, got %d", len(hash1))
	}

	hash3 := sha256Hex("hello WORLD")
	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}
}

func TestSha256Hex_KnownValue(t *testing.T) {
	// SHA256("hello world") = b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	got := sha256Hex("hello world")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

// ─── extractTraceID 测试 ───────────────────────────────────────────────────

func TestExtractTraceID_FromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "abc-123")
	traceID := extractTraceID(ctx)
	if traceID != "abc-123" {
		t.Errorf("expected abc-123, got %s", traceID)
	}
}

func TestExtractTraceID_Missing(t *testing.T) {
	ctx := context.Background()
	traceID := extractTraceID(ctx)
	if traceID == "" {
		t.Error("missing trace_id should generate fallback, not empty")
	}
	if len(traceID) < 5 {
		t.Errorf("fallback trace_id too short: %s", traceID)
	}
}

// ─── nextAIAuditLogID 测试 ─────────────────────────────────────────────────

func TestNextAIAuditLogID(t *testing.T) {
	id1 := nextAIAuditLogID()
	id2 := nextAIAuditLogID()
	if id1 == id2 {
		t.Error("consecutive IDs should differ")
	}
	if id1 <= 0 || id2 <= 0 {
		t.Error("IDs should be positive")
	}
}