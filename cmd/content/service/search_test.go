package service

import (
	"strings"
	"testing"

	pb "go_projects/praProject1/PB/pb/content_pb"
	common_pb "go_projects/praProject1/PB/pb/common_pb"
)

// ─── buildSearchQuery 查询构造测试 ─────────────────────────────────────────────

func TestBuildSearchQuery_Basic(t *testing.T) {
	query := buildSearchQuery(1, "测试关键词", pb.PostType_POST_TYPE_UNSPECIFIED,
		pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED, pb.PostStatus_POST_STATUS_UNSPECIFIED,
		0, 20, common_pb.SortType_SORT_TYPE_UNSPECIFIED)

	checks := []string{
		`"from":0`,
		`"size":20`,
		`"multi_match"`,
		`"测试关键词"`,
		`"title","content"`,
		`"school_id":1`,
		`"created_at"`,
	}
	for _, check := range checks {
		if !strings.Contains(query, check) {
			t.Errorf("查询应包含 %s，实际: %s", check, query)
		}
	}
}

func TestBuildSearchQuery_WithTypeFilter(t *testing.T) {
	query := buildSearchQuery(1, "手机", pb.PostType_POST_TYPE_SECOND_HAND,
		pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED, pb.PostStatus_POST_STATUS_UNSPECIFIED,
		0, 10, common_pb.SortType_SORT_TYPE_TIME_DESC)

	if !strings.Contains(query, `"type":3`) {
		t.Errorf("查询应包含二手分类筛选，实际: %s", query)
	}
}

func TestBuildSearchQuery_WithStatusFilter(t *testing.T) {
	query := buildSearchQuery(2, "test", pb.PostType_POST_TYPE_UNSPECIFIED,
		pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED, pb.PostStatus_POST_STATUS_PUBLISHED,
		0, 20, common_pb.SortType_SORT_TYPE_UNSPECIFIED)

	if !strings.Contains(query, `"status":2`) {
		t.Errorf("查询应包含状态筛选（published=2），实际: %s", query)
	}
}

func TestBuildSearchQuery_Pagination(t *testing.T) {
	query := buildSearchQuery(1, "测试", pb.PostType_POST_TYPE_UNSPECIFIED,
		pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED, pb.PostStatus_POST_STATUS_UNSPECIFIED,
		20, 15, common_pb.SortType_SORT_TYPE_UNSPECIFIED)

	if !strings.Contains(query, `"from":20`) {
		t.Errorf("查询应包含 from=20，实际: %s", query)
	}
	if !strings.Contains(query, `"size":15`) {
		t.Errorf("查询应包含 size=15，实际: %s", query)
	}
}

func TestBuildSearchQuery_SortTypes(t *testing.T) {
	tests := []struct {
		name       string
		sort       common_pb.SortType
		expectSort string
	}{
		{"时间倒序", common_pb.SortType_SORT_TYPE_TIME_DESC, "created_at"},
		{"点赞倒序", common_pb.SortType_SORT_TYPE_LIKES_DESC, "likes_count"},
		{"相关度", common_pb.SortType_SORT_TYPE_RELEVANCE, "_score"},
		{"默认（未指定）", common_pb.SortType_SORT_TYPE_UNSPECIFIED, "created_at"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := buildSearchQuery(1, "test", pb.PostType_POST_TYPE_UNSPECIFIED,
				pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED, pb.PostStatus_POST_STATUS_UNSPECIFIED,
				0, 10, tc.sort)

			if !strings.Contains(query, tc.expectSort) {
				t.Errorf("sort=%v 应包含 %s，实际: %s", tc.sort, tc.expectSort, query)
			}
		})
	}
}

func TestBuildSearchQuery_WithCategory(t *testing.T) {
	query := buildSearchQuery(1, "test", pb.PostType_POST_TYPE_UNSPECIFIED,
		pb.ItemCategory(1), pb.PostStatus_POST_STATUS_UNSPECIFIED,
		0, 20, common_pb.SortType_SORT_TYPE_UNSPECIFIED)

	if !strings.Contains(query, "should") {
		t.Errorf("分类筛选应使用 bool should 查询，实际: %s", query)
	}
	if !strings.Contains(query, "lf_category") || !strings.Contains(query, "sh_category") {
		t.Errorf("分类筛选应同时匹配 lf_category 和 sh_category，实际: %s", query)
	}
}

// ─── buildSearchQuery 特殊字符处理测试 ─────────────────────────────────────────

func TestBuildSearchQuery_SpecialChars(t *testing.T) {
	// 关键词含引号应被转义
	query := buildSearchQuery(1, "test\"keyword", pb.PostType_POST_TYPE_UNSPECIFIED,
		pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED, pb.PostStatus_POST_STATUS_UNSPECIFIED,
		0, 10, common_pb.SortType_SORT_TYPE_UNSPECIFIED)

	if !strings.Contains(query, "multi_match") {
		t.Error("查询应包含 multi_match")
	}
}
