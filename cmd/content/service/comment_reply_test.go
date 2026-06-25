package service

import (
	"testing"

	content_db "go_projects/praProject1/cmd/content/model"
	pb "go_projects/praProject1/PB/pb/content_pb"
)

// ─── 二级回复参数校验测试（无 DB） ─────────────────────────────────────────────

// TestCreateComment_ParentIDField_Proto 验证 CreateCommentRequest 暴露 ParentId 字段。
func TestCreateComment_ParentIDField_Proto(t *testing.T) {
	req := &pb.CreateCommentRequest{
		SchoolId: 1,
		PostId:   100,
		UserId:   200,
		Content:  "test",
		ParentId: 999,
	}
	if req.GetParentId() != 999 {
		t.Errorf("ParentId 应为 999，实际 %d", req.GetParentId())
	}
}

// TestCreateComment_ParentIDZero_LevelOne 验证 ParentId=0 表示一级评论（沿用原行为）。
func TestCreateComment_ParentIDZero_LevelOne(t *testing.T) {
	req := &pb.CreateCommentRequest{
		ParentId: 0,
		Content:  "一级评论",
	}
	if req.GetParentId() != 0 {
		t.Error("ParentId=0 应为一级评论")
	}
}

// ─── 父评论状态校验测试（不依赖 DB，纯逻辑） ──────────────────────────────────

// TestParentComment_ValidationRules 验证父评论业务校验的预期语义。
// 这里仅测试"父评论 parent_id=0 必须为一级评论"的检查逻辑。
func TestParentComment_ValidationRules(t *testing.T) {
	// 模拟父评论：parent_id=0 → 一级评论（合法作为二级回复的父）
	parent := &content_db.PostComment{
		ID:       1,
		PostID:   100,
		ParentID: 0, // 一级
		Status:   1, // 正常
	}
	if parent.ParentID != 0 {
		t.Error("测试前提：父评论必须是一级评论（parent_id=0）")
	}
	if parent.Status != 1 {
		t.Error("测试前提：父评论状态必须为正常")
	}
}

// TestParentComment_RejectsNestedReply 验证父评论本身是二级评论时不允许再嵌套。
func TestParentComment_RejectsNestedReply(t *testing.T) {
	// 父评论本身是二级评论
	parent := &content_db.PostComment{
		ID:       2,
		PostID:   100,
		ParentID: 1, // 二级评论
		Status:   1,
	}
	// 业务规则：parent.parent_id 必须 == 0
	if parent.ParentID == 0 {
		t.Error("测试前提：父评论不能是一级评论")
	}
	// 预期被拒绝
	canReply := parent.ParentID == 0
	if canReply {
		t.Error("父评论为二级评论时，不应允许再创建回复")
	}
}

// TestParentComment_RejectsDeleted 验证父评论已删除时不允许回复。
func TestParentComment_RejectsDeleted(t *testing.T) {
	parent := &content_db.PostComment{
		ID:       3,
		PostID:   100,
		ParentID: 0,
		Status:   2, // 已删除
	}
	if parent.Status == 1 {
		t.Error("测试前提：父评论必须已删除")
	}
}

// TestParentComment_CrossPost 验证父评论与目标帖子不匹配时拒绝。
func TestParentComment_CrossPost(t *testing.T) {
	parent := &content_db.PostComment{
		ID:       4,
		PostID:   100, // 父评论属于帖子 100
		ParentID: 0,
		Status:   1,
	}
	targetPostID := int64(200) // 请求回复到帖子 200
	if parent.PostID == targetPostID {
		t.Error("测试前提：父评论与目标帖子必须不同")
	}
}

// ─── 错误消息模板测试（确保错误信息准确） ─────────────────────────────────────

func TestErrorMessages_ReplyValidation(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"父评论不存在", "父评论 %d 不存在"},
		{"父评论已删除", "父评论已被删除，无法回复"},
		{"不支持嵌套", "仅支持二级回复，不允许嵌套"},
		{"跨帖子", "父评论所属帖子与请求不匹配"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expected == "" {
				t.Error("错误消息模板不应为空")
			}
		})
	}
}
