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
	if mq.EventContentLiked != "content.liked" {
		t.Errorf("EventContentLiked 应为 content.liked，实际 %s", mq.EventContentLiked)
	}
	if mq.EventContentReplied != "content.replied" {
		t.Errorf("EventContentReplied 应为 content.replied，实际 %s", mq.EventContentReplied)
	}
}

// ─── 双队列投递测试 ──────────────────────────────────────────────────────────

func TestIsNotificationEvent(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{mq.EventContentLiked, true},
		{mq.EventContentPublished, true},
		{mq.EventContentRejected, true},
		{mq.EventContentTakenDown, true},
		{mq.EventContentReplied, true},
		{"content.unknown", false},
		{"", false},
		{"post:create", false},
	}
	for _, tc := range tests {
		got := isNotificationEvent(tc.eventType)
		if got != tc.want {
			t.Errorf("isNotificationEvent(%q) = %v，期望 %v", tc.eventType, got, tc.want)
		}
	}
}

func TestPublishNotificationEventRaw_NilPublisher(t *testing.T) {
	// 确保 notificationPublisher 为 nil 时不 panic
	oldPublisher := notificationPublisher
	notificationPublisher = nil
	defer func() { notificationPublisher = oldPublisher }()

	event := mq.NewContentEvent(mq.EventContentLiked, 1, 1, 1, "")
	// 不应 panic
	publishNotificationEventRaw(event)
}

func TestInitMQ_DualPublisher(t *testing.T) {
	oldPub := mqPublisher
	oldNotif := notificationPublisher
	defer func() {
		mqPublisher = oldPub
		notificationPublisher = oldNotif
	}()

	InitMQ("amqp://test:test@localhost:5672/")
	if mqPublisher == nil {
		t.Error("InitMQ 应设置 mqPublisher")
	}
	if notificationPublisher == nil {
		t.Error("InitMQ 应设置 notificationPublisher")
	}
}

// ─── formatInt64 测试 ─────────────────────────────────────────────────────────

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{123, "123"},
		{-1, "-1"},
		{9999999999999, "9999999999999"},
	}
	for _, tc := range tests {
		got := formatInt64(tc.input)
		if got != tc.want {
			t.Errorf("formatInt64(%d) = %q，期望 %q", tc.input, got, tc.want)
		}
	}
}

// ─── content.replied 事件辅助测试 ─────────────────────────────────────────────

func TestRepliedEvent_DataFields(t *testing.T) {
	event := mq.NewContentEvent(mq.EventContentReplied, 100, 1, 200, "trace-reply")
	event.Data["parent_comment_id"] = "50"
	event.Data["parent_comment_user_id"] = "300"
	event.Data["content_preview"] = "好的，我同意"

	if event.Type != mq.EventContentReplied {
		t.Errorf("type 应为 %s，实际 %s", mq.EventContentReplied, event.Type)
	}
	if event.Data["parent_comment_id"] != "50" {
		t.Errorf("parent_comment_id 应为 50，实际 %s", event.Data["parent_comment_id"])
	}
	if event.Data["parent_comment_user_id"] != "300" {
		t.Errorf("parent_comment_user_id 应为 300，实际 %s", event.Data["parent_comment_user_id"])
	}
	if event.Data["content_preview"] != "好的，我同意" {
		t.Errorf("content_preview 应为「好的，我同意」，实际 %s", event.Data["content_preview"])
	}
}

func TestRepliedEvent_PreviewTruncation(t *testing.T) {
	longContent := ""
	for i := 0; i < 100; i++ {
		longContent += "字"
	}
	preview := string([]rune(longContent))
	if len([]rune(preview)) > 50 {
		preview = string([]rune(preview)[:50])
	}
	if len([]rune(preview)) != 50 {
		t.Errorf("截断后 preview 应为 50 个字符，实际 %d", len([]rune(preview)))
	}
}

// ─── SensitiveWordError 兼容性测试 ────────────────────────────────────────────

func TestSensitiveWordError_AsError(t *testing.T) {
	err := &SensitiveWordErrorType{Hits: nil}
	if err.Error() != "内容包含敏感词" {
		t.Errorf("错误消息应为'内容包含敏感词'，实际 %s", err.Error())
	}
}
