package service

import (
	"testing"

	"go_projects/praProject1/cmd/message/model"
	"go_projects/praProject1/pkg/mq"
)

// ─── FormatReviewTitle ─────────────────────────────────────────────────────

func TestFormatReviewTitle_Published(t *testing.T) {
	title, notifType := FormatReviewTitle(mq.EventContentPublished, "")
	if title != "你的帖子已通过审核" {
		t.Errorf("审核通过标题应为「你的帖子已通过审核」，实际 %q", title)
	}
	if notifType != string(model.NotifPublished) {
		t.Errorf("通知类型应为 published，实际 %s", notifType)
	}
}

func TestFormatReviewTitle_Rejected(t *testing.T) {
	title, notifType := FormatReviewTitle(mq.EventContentRejected, "内容违规")
	if title != "你的帖子审核未通过，原因: 内容违规" {
		t.Errorf("审核拒绝标题错误，实际 %q", title)
	}
	if notifType != string(model.NotifReviewResult) {
		t.Errorf("通知类型应为 review_result，实际 %s", notifType)
	}
}

func TestFormatReviewTitle_RejectedEmptyReason(t *testing.T) {
	title, _ := FormatReviewTitle(mq.EventContentRejected, "")
	if title != "你的帖子审核未通过，原因: 未提供原因" {
		t.Errorf("空原因应使用默认文案，实际 %q", title)
	}
}

// ─── FormatTakenDownTitle ───────────────────────────────────────────────────

func TestFormatTakenDownTitle_WithReason(t *testing.T) {
	title := FormatTakenDownTitle("广告内容")
	if title != "你的帖子因违规已下架，原因: 广告内容" {
		t.Errorf("下架标题错误，实际 %q", title)
	}
}

func TestFormatTakenDownTitle_EmptyReason(t *testing.T) {
	title := FormatTakenDownTitle("")
	if title != "你的帖子因违规已下架，原因: 未提供原因" {
		t.Errorf("空原因应使用默认文案，实际 %q", title)
	}
}

// ─── FormatRepliedTitle ─────────────────────────────────────────────────────

func TestFormatRepliedTitle_WithPreview(t *testing.T) {
	title := FormatRepliedTitle("我也觉得")
	if title != "有人回复了你的评论: 我也觉得" {
		t.Errorf("回复标题错误，实际 %q", title)
	}
}

func TestFormatRepliedTitle_EmptyPreview(t *testing.T) {
	title := FormatRepliedTitle("")
	if title != "有人回复了你的评论: 回复了你的评论" {
		t.Errorf("空 preview 应使用默认文案，实际 %q", title)
	}
}
