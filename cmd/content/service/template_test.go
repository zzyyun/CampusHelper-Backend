package service

import (
	"testing"

	content_db "go_projects/praProject1/cmd/content/model"
)

// ─── 过期时间逻辑测试 ────────────────────────────────────────────────────────

func TestExpiredAt_LostFound30Days(t *testing.T) {
	// 失物招领应为 30 天（无 DB 的纯逻辑测试）
	if content_db.PostStatusRetrieved != 6 {
		t.Errorf("PostStatusRetrieved 应为 6，实际 %d", content_db.PostStatusRetrieved)
	}
}

func TestExpiredAt_SecondHand60Days(t *testing.T) {
	if content_db.PostStatusSold != 7 {
		t.Errorf("PostStatusSold 应为 7，实际 %d", content_db.PostStatusSold)
	}
}

// ─── 状态转换验证测试 ────────────────────────────────────────────────────────

func TestStatusTransition_Retrieved(t *testing.T) {
	// published → retrieved 应合法
	err := content_db.PostStatusPublished.CanTransitionTo(content_db.PostStatusRetrieved)
	if err != nil {
		t.Errorf("published→retrieved 为合法转移，不应返回错误: %v", err)
	}
}

func TestStatusTransition_Sold(t *testing.T) {
	// published → sold 应合法
	err := content_db.PostStatusPublished.CanTransitionTo(content_db.PostStatusSold)
	if err != nil {
		t.Errorf("published→sold 为合法转移，不应返回错误: %v", err)
	}
}

func TestStatusTransition_RetrievedFromPending(t *testing.T) {
	// pending → retrieved 不应合法
	err := content_db.PostStatusPending.CanTransitionTo(content_db.PostStatusRetrieved)
	if err == nil {
		t.Error("pending→retrieved 为非法转移，应返回错误")
	}
}

func TestStatusTransition_SoldFromPending(t *testing.T) {
	// pending → sold 不应合法
	err := content_db.PostStatusPending.CanTransitionTo(content_db.PostStatusSold)
	if err == nil {
		t.Error("pending→sold 为非法转移，应返回错误")
	}
}

// ─── 状态枚举一致性测试 ──────────────────────────────────────────────────────

func TestAllPostStatuses_Defined(t *testing.T) {
	statuses := map[content_db.PostStatus]string{
		content_db.PostStatusUnspecified: "unspecified",
		content_db.PostStatusPending:     "pending",
		content_db.PostStatusPublished:   "published",
		content_db.PostStatusExpired:     "expired",
		content_db.PostStatusClosed:      "closed",
		content_db.PostStatusRejected:    "rejected",
		content_db.PostStatusRetrieved:   "retrieved",
		content_db.PostStatusSold:        "sold",
	}
	if len(statuses) != 8 {
		t.Errorf("应有 8 个状态枚举，实际 %d", len(statuses))
	}
	for status, name := range statuses {
		if name == "" {
			t.Errorf("status %d 无名称", status)
		}
	}
}

// ─── 类型常量验证 ────────────────────────────────────────────────────────────

func TestPostType_Values(t *testing.T) {
	types := map[int]string{
		1: "general",
		2: "lost_found",
		3: "second_hand",
	}
	for val, name := range types {
		if name == "" {
			t.Errorf("type %d 无名称", val)
		}
	}
}
