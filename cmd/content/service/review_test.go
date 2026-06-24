package service

import (
	"testing"

	content_db "go_projects/praProject1/cmd/content/model"
	pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/pkg/mq"
)

// ─── MQ 事件辅助函数测试 ──────────────────────────────────────────────────────

func TestNewContentEvent_Type(t *testing.T) {
	event := mq.NewContentEvent(mq.EventContentPublished, 100, 1, 200, "trace-abc")
	if event.Type != mq.EventContentPublished {
		t.Errorf("事件类型应为 %s，实际 %s", mq.EventContentPublished, event.Type)
	}
	if event.PostID != 100 {
		t.Errorf("PostID 应为 100，实际 %d", event.PostID)
	}
	if event.SchoolID != 1 {
		t.Errorf("SchoolID 应为 1，实际 %d", event.SchoolID)
	}
	if event.UserID != 200 {
		t.Errorf("UserID 应为 200，实际 %d", event.UserID)
	}
	if event.TraceID != "trace-abc" {
		t.Errorf("TraceID 应为 trace-abc，实际 %s", event.TraceID)
	}
	if event.Time == "" {
		t.Error("事件时间不应为空")
	}
	if event.Data == nil {
		t.Error("Data map 应为空 map 而非 nil")
	}
}

func TestNewContentEvent_RejectedWithData(t *testing.T) {
	event := mq.NewContentEvent(mq.EventContentRejected, 200, 2, 300, "")
	event.Data["result"] = "rejected"
	event.Data["reason"] = "内容违规"

	if event.Data["result"] != "rejected" {
		t.Errorf("result 应为 rejected，实际 %s", event.Data["result"])
	}
	if event.Data["reason"] != "内容违规" {
		t.Errorf("reason 应为内容违规，实际 %s", event.Data["reason"])
	}
}

func TestPublishEventRaw_NilPublisher(t *testing.T) {
	// 确保 mqPublisher 为 nil 时不 panic
	oldPublisher := mqPublisher
	mqPublisher = nil
	defer func() { mqPublisher = oldPublisher }()

	event := mq.NewContentEvent(mq.EventContentPublished, 1, 1, 1, "")
	// 不应 panic，应优雅降级
	publishEventRaw(event) // panic 则测试失败
}

func TestPublishEvent_NilPublisher(t *testing.T) {
	oldPublisher := mqPublisher
	mqPublisher = nil
	defer func() { mqPublisher = oldPublisher }()

	// publishEvent 在 mqPublisher 为 nil 时不应 panic
	publishEvent(nil, mq.EventContentPublished, 1, 1, 1)
}

func TestInitMQ(t *testing.T) {
	oldPublisher := mqPublisher
	defer func() { mqPublisher = oldPublisher }()

	InitMQ("amqp://test:test@localhost:5672/")
	if mqPublisher == nil {
		t.Error("InitMQ 应设置 mqPublisher")
	}
}

// ─── 状态机一致性测试 ────────────────────────────────────────────────────────

func TestReviewStatusTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    content_db.PostStatus
		to      content_db.PostStatus
		wantErr bool
	}{
		// 合法转移
		{"审核通过", content_db.PostStatusPending, content_db.PostStatusPublished, false},
		{"审核拒绝", content_db.PostStatusPending, content_db.PostStatusRejected, false},
		{"违规下架", content_db.PostStatusPublished, content_db.PostStatusClosed, false},
		// 非法转移
		{"不允许: published→published(下架)", content_db.PostStatusPublished, content_db.PostStatusPublished, false}, // 幂等
		{"不允许: rejected→published", content_db.PostStatusRejected, content_db.PostStatusPublished, true},
		{"不允许: closed→published", content_db.PostStatusClosed, content_db.PostStatusPublished, true},
		{"不允许: pending→closed", content_db.PostStatusPending, content_db.PostStatusClosed, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.CanTransitionTo(tc.to)
			if tc.wantErr && err == nil {
				t.Errorf("期望错误但返回 nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("不期望错误但返回: %v", err)
			}
		})
	}
}

// ─── Proto 类型映射测试 ──────────────────────────────────────────────────────

func TestPostStatus_ProtoMapping(t *testing.T) {
	// 验证 model.PostStatus 与 pb.PostStatus 枚举值一致
	mapping := []struct {
		model content_db.PostStatus
		pb    pb.PostStatus
	}{
		{content_db.PostStatusPending, pb.PostStatus_POST_STATUS_PENDING},
		{content_db.PostStatusPublished, pb.PostStatus_POST_STATUS_PUBLISHED},
		{content_db.PostStatusRejected, pb.PostStatus_POST_STATUS_REJECTED},
		{content_db.PostStatusClosed, pb.PostStatus_POST_STATUS_CLOSED},
	}
	for _, m := range mapping {
		if int32(m.model) != int32(m.pb) {
			t.Errorf("model.PostStatus(%d) != pb.PostStatus(%d)", m.model, m.pb)
		}
	}
}

// ─── MQ 事件常量测试 ──────────────────────────────────────────────────────────

func TestMQEventConstants(t *testing.T) {
	if mq.EventContentPublished != "content.published" {
		t.Errorf("EventContentPublished 应为 content.published，实际 %s", mq.EventContentPublished)
	}
	if mq.EventContentRejected != "content.review_result" {
		t.Errorf("EventContentRejected 应为 content.review_result，实际 %s", mq.EventContentRejected)
	}
	if mq.EventContentTakenDown != "content.taken_down" {
		t.Errorf("EventContentTakenDown 应为 content.taken_down，实际 %s", mq.EventContentTakenDown)
	}
}

// ─── SensitiveWordError 兼容性测试 ────────────────────────────────────────────

func TestSensitiveWordError_AsError(t *testing.T) {
	err := &SensitiveWordErrorType{Hits: nil}
	if err.Error() != "内容包含敏感词" {
		t.Errorf("错误消息应为'内容包含敏感词'，实际 %s", err.Error())
	}
}
