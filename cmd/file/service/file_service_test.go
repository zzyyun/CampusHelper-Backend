package service

import (
	"testing"

	"go_projects/praProject1/cmd/file/model"
)

func TestIsAllowedType(t *testing.T) {
	allowed := []string{"image/jpeg", "image/png", "image/webp"}
	tests := []struct {
		contentType string
		want        bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/webp", true},
		{"image/gif", false},
		{"application/pdf", false},
		{"text/html", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isAllowedType(tc.contentType, allowed)
		if got != tc.want {
			t.Errorf("isAllowedType(%q) = %v, 期望 %v", tc.contentType, got, tc.want)
		}
	}
}

func TestExtFromContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"unknown/type", ""}, // path.Ext("unknown/type") returns ""
	}
	for _, tc := range tests {
		got := extFromContentType(tc.contentType)
		if got != tc.want {
			t.Errorf("extFromContentType(%q) = %q, 期望 %q", tc.contentType, got, tc.want)
		}
	}
}

func TestNormalizeCategory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"avatar", "avatar"},
		{"post", "post"},
		{"task", "task"},
		{"other", "other"},
		{"invalid", "other"},
		{"", "other"},
	}
	for _, tc := range tests {
		got := normalizeCategory(tc.input)
		if got != tc.want {
			t.Errorf("normalizeCategory(%q) = %q, 期望 %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildStorageKey(t *testing.T) {
	key := buildStorageKey("post", "abcdef0123456789...", "image/png")
	if !contains(key, "post/") {
		t.Errorf("storage key 应包含 post/，实际 %s", key)
	}
	if !contains(key, ".png") {
		t.Errorf("storage key 应包含 .png 后缀，实际 %s", key)
	}
}

func TestToPbFileInfo(t *testing.T) {
	f := &model.File{
		SchoolID:    100,
		UploaderID:  1,
		Category:    "post",
		URL:         "http://minio/test.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
		SHA256:      "abcdef",
	}
	// CreatedAt 默认为零值
	pb := toPbFileInfo(f)
	if pb.UploaderId != 1 || pb.Category != "post" {
		t.Errorf("转换错误: %+v", pb)
	}
}

// contains is a simple helper
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var _ = model.FileCategoryOther // 确保 model 包被引用