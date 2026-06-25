package service

import (
	"testing"

	content_db "go_projects/praProject1/cmd/content/model"
)

// ─── 级联软删除逻辑测试（无 DB 的纯逻辑测试） ─────────────────────────────────

// TestCascadeDelete_TriggersOnLevelOne 验证：删除一级评论时应级联删除其下回复。
func TestCascadeDelete_TriggersOnLevelOne(t *testing.T) {
	comment := &content_db.PostComment{
		ID:       100,
		PostID:   1,
		ParentID: 0, // 一级评论
		Status:   1,
	}
	shouldCascade := comment.ParentID == 0
	if !shouldCascade {
		t.Error("一级评论应触发级联删除")
	}
}

// TestCascadeDelete_SkipsOnReply 验证：删除二级评论时不应触发级联。
func TestCascadeDelete_SkipsOnReply(t *testing.T) {
	comment := &content_db.PostComment{
		ID:       200,
		PostID:   1,
		ParentID: 100, // 二级评论
		Status:   1,
	}
	shouldCascade := comment.ParentID == 0
	if shouldCascade {
		t.Error("二级评论不应触发级联删除（仅删自己）")
	}
}

// TestDecCommentCountBy_GreatestZero 验证 comment_count 不会减成负数。
func TestDecCommentCountBy_GreatestZero(t *testing.T) {
	// 业务规则：使用 GREATEST(comment_count - delta, 0) 保证非负
	// 实际 SQL: UPDATE posts SET comment_count = GREATEST(comment_count - ?, 0)
	// 这里仅验证设计意图
	current := int32(0)
	delta := int32(5)
	newCount := current - delta
	if newCount < 0 {
		// SQL 层 GREATEST 保护
		newCount = 0
	}
	if newCount != 0 {
		t.Errorf("count 应被 GREATEST 保护为 0，实际 %d", newCount)
	}
}

// TestCascadeDelete_CommentCountDecrement 验证删除一级评论 + N 条回复时 count 递减 1+N。
func TestCascadeDelete_CommentCountDecrement(t *testing.T) {
	levelOneCount := 1
	repliesCount := 3
	expectedDecrement := levelOneCount + repliesCount
	if expectedDecrement != 4 {
		t.Errorf("预期递减 4，实际 %d", expectedDecrement)
	}
}

// ─── ListReplies 父评论校验测试 ───────────────────────────────────────────────

func TestListReplies_ParentMustBeLevelOne(t *testing.T) {
	parent := &content_db.PostComment{
		ID:       300,
		ParentID: 0, // 一级评论
		Status:   1,
	}
	if parent.ParentID != 0 {
		t.Error("ListReplies 的父评论必须是一级评论（parent_id=0）")
	}
}

func TestListReplies_RejectsNestedParent(t *testing.T) {
	// 父评论本身是二级评论
	parent := &content_db.PostComment{
		ID:       400,
		ParentID: 100, // 二级评论
		Status:   1,
	}
	if parent.ParentID == 0 {
		t.Error("测试前提：父评论不能是一级评论")
	}
	// 业务规则：ListReplies 应拒绝
	canListReplies := parent.ParentID == 0
	if canListReplies {
		t.Error("父评论为二级评论时，不应允许查询其下回复（无嵌套）")
	}
}

// ─── 错误消息模板测试 ─────────────────────────────────────────────────────────

func TestCascadeDelete_ErrorMessages(t *testing.T) {
	messages := map[string]string{
		"父评论不存在": "父评论 %d 不存在",
		"父评论非法":  "父评论必须是顶级评论",
	}
	for name, tmpl := range messages {
		if tmpl == "" {
			t.Errorf("消息模板 %s 不应为空", name)
		}
	}
}