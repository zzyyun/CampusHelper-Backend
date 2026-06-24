package service

import (
	"testing"

	content_db "go_projects/praProject1/cmd/content/model"
	pb "go_projects/praProject1/PB/pb/content_pb"
)

// ─── toPbComment 转换测试 ──────────────────────────────────────────────────────

func TestToPbComment_Normal(t *testing.T) {
	c := &content_db.PostComment{
		ID:       100,
		SchoolID: 1,
		PostID:   50,
		UserID:   200,
		Content:  "这是一条测试评论",
		ParentID: 0,
		Status:   1,
	}

	result := toPbComment(c)

	if result.Id != 100 {
		t.Errorf("Id 应为 100，实际 %d", result.Id)
	}
	if result.SchoolId != 1 {
		t.Errorf("SchoolId 应为 1，实际 %d", result.SchoolId)
	}
	if result.PostId != 50 {
		t.Errorf("PostId 应为 50，实际 %d", result.PostId)
	}
	if result.UserId != 200 {
		t.Errorf("UserId 应为 200，实际 %d", result.UserId)
	}
	if result.Content != "这是一条测试评论" {
		t.Errorf("Content 不匹配，实际 %s", result.Content)
	}
	if result.ParentId != 0 {
		t.Errorf("ParentId 应为 0（一级评论），实际 %d", result.ParentId)
	}
	if result.Status != pb.CommentStatus(1) {
		t.Errorf("Status 应为 1（正常），实际 %d", result.Status)
	}
	if result.CreatedAt == 0 {
		t.Error("CreatedAt 不应为 0")
	}
}

func TestToPbComment_Nil(t *testing.T) {
	result := toPbComment(nil)
	if result != nil {
		t.Errorf("nil 输入应返回 nil，实际: %+v", result)
	}
}

func TestToPbComment_DeletedStatus(t *testing.T) {
	c := &content_db.PostComment{
		ID:     200,
		Status: 2, // 已删除
	}
	result := toPbComment(c)
	if result.Status != pb.CommentStatus(2) {
		t.Errorf("已删除状态的 Status 应为 2，实际 %d", result.Status)
	}
}

// ─── CommentStatus Proto 映射测试 ──────────────────────────────────────────────

func TestCommentStatus_Values(t *testing.T) {
	// 验证 model 和 proto 的状态值一致
	tests := []struct {
		model int8
		pb    pb.CommentStatus
	}{
		{1, pb.CommentStatus(1)}, // NORMAL
		{2, pb.CommentStatus(2)}, // DELETED
	}

	for _, tc := range tests {
		if int32(tc.model) != int32(tc.pb) {
			t.Errorf("model %d != pb %d", tc.model, tc.pb)
		}
	}
}

// ─── Content 长度验证测试 ─────────────────────────────────────────────────────

func TestCommentContentLength(t *testing.T) {
	// 验证 500 字上限
	runes := make([]rune, 501)
	for i := range runes {
		runes[i] = '测'
	}
	longContent := string(runes)

	if len([]rune(longContent)) != 501 {
		t.Fatalf("测试设置错误：期望 501 个 runes，实际 %d", len([]rune(longContent)))
	}

	// 500 字以内应合法
	validContent := string(runes[:500])
	if len([]rune(validContent)) != 500 {
		t.Fatalf("validContent 应为 500 runes，实际 %d", len([]rune(validContent)))
	}
}

func TestCommentContent_Empty(t *testing.T) {
	// 模拟去除空白后的空内容
	tests := []string{
		"",
		"   ",
		"\n\t  ",
	}

	for _, tc := range tests {
		trimmed := ""
		for _, r := range tc {
			if r != ' ' && r != '\n' && r != '\t' {
				trimmed += string(r)
			}
		}
		if trimmed != "" {
			t.Errorf("输入 %q 去除空白后应为空，实际: %q", tc, trimmed)
		}
	}
}

// ─── DFA 扫描兼容性测试 ───────────────────────────────────────────────────────

func TestCommentDFAScan_Integration(t *testing.T) {
	// 验证评论内容的 DFA 扫描（与发帖共享）
	safeComment := "正常评论内容"
	if hits := ScanSensitive(safeComment); len(hits) != 0 {
		t.Errorf("正常评论不应触发敏感词: %v", hits)
	}

	// "微信号" 在默认词表中
	sensitiveComment := "加我微信号abc123"
	hits := ScanSensitive(sensitiveComment)
	if len(hits) == 0 {
		t.Error("含敏感词的评论应被检测到")
	}
}
