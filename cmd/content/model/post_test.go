package model

import (
	"testing"
)

// TestPostType_IsValid 测试 PostType 枚举值校验。
func TestPostType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		pt    PostType
		valid bool
	}{
		{"通用帖子", PostTypeGeneral, true},
		{"失物招领", PostTypeLostFound, true},
		{"二手交易", PostTypeSecondHand, true},
		{"未指定(0)", PostType(0), false},
		{"非法值(99)", PostType(99), false},
		{"负数", PostType(-1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pt.IsValid(); got != tt.valid {
				t.Errorf("PostType(%d).IsValid() = %v, want %v", tt.pt, got, tt.valid)
			}
		})
	}
}

// TestPostStatus_IsValid 测试 PostStatus 枚举值校验。
func TestPostStatus_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		ps    PostStatus
		valid bool
	}{
		{"审核中", PostStatusPending, true},
		{"已发布", PostStatusPublished, true},
		{"已过期", PostStatusExpired, true},
		{"已关闭", PostStatusClosed, true},
		{"已拒绝", PostStatusRejected, true},
		{"已当领", PostStatusRetrieved, true},
		{"已售出", PostStatusSold, true},
		{"未指定(0)", PostStatus(0), false},
		{"非法值(99)", PostStatus(99), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ps.IsValid(); got != tt.valid {
				t.Errorf("PostStatus(%d).IsValid() = %v, want %v", tt.ps, got, tt.valid)
			}
		})
	}
}

// TestCanTransitionTo 测试状态机流转合法性。
func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from PostStatus
		to   PostStatus
		want bool
	}{
		// 合法流转
		{"pending→published(审核通过)", PostStatusPending, PostStatusPublished, true},
		{"pending→rejected(审核拒绝)", PostStatusPending, PostStatusRejected, true},
		{"pending→closed(用户撤回)", PostStatusPending, PostStatusClosed, true},
		{"published→expired(自然过期)", PostStatusPublished, PostStatusExpired, true},
		{"published→closed(用户关闭)", PostStatusPublished, PostStatusClosed, true},
		{"published→retrieved(失物已当领)", PostStatusPublished, PostStatusRetrieved, true},
		{"published→sold(二手已售出)", PostStatusPublished, PostStatusSold, true},
		// 非法流转
		{"pending→expired(跳过审核)", PostStatusPending, PostStatusExpired, false},
		{"pending→retrieved", PostStatusPending, PostStatusRetrieved, false},
		{"published→pending(回退)", PostStatusPublished, PostStatusPending, false},
		{"published→rejected", PostStatusPublished, PostStatusRejected, false},
		{"rejected→published(重新发布)", PostStatusRejected, PostStatusPublished, false},
		{"closed→published(复活)", PostStatusClosed, PostStatusPublished, false},
		{"expired→published(复活)", PostStatusExpired, PostStatusPublished, false},
		{"retrieved→published", PostStatusRetrieved, PostStatusPublished, false},
		{"sold→published", PostStatusSold, PostStatusPublished, false},
		// 自身流转
		{"published→published", PostStatusPublished, PostStatusPublished, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransitionTo(tt.from, tt.to); got != tt.want {
				t.Errorf("CanTransitionTo(%d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// TestStringArray_Value 测试 StringArray 的 JSON 序列化。
func TestStringArray_Value(t *testing.T) {
	tests := []struct {
		name  string
		input StringArray
		want  string
	}{
		{"空数组", StringArray{}, "[]"},
		{"nil 数组", nil, "[]"},
		{"单元素", StringArray{"a"}, `["a"]`},
		{"多元素", StringArray{"a", "b", "c"}, `["a","b","c"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.Value()
			if err != nil {
				t.Fatalf("Value() error = %v", err)
			}
			// got 可能是 string 或 []byte，统一转为 string 比较
			var gotStr string
			switch v := got.(type) {
			case string:
				gotStr = v
			case []byte:
				gotStr = string(v)
			default:
				t.Fatalf("Value() returned %T, want string or []byte", got)
			}
			if gotStr != tt.want {
				t.Errorf("Value() = %s, want %s", gotStr, tt.want)
			}
		})
	}
}

// TestStringArray_Scan 测试 StringArray 的 JSON 反序列化。
func TestStringArray_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    StringArray
		wantErr bool
	}{
		{"nil 值", nil, nil, false},
		{"空字符串", "", []string{}, false},
		{"JSON 字节数组", []byte(`["a","b"]`), StringArray{"a", "b"}, false},
		{"JSON 字符串", `["x","y"]`, StringArray{"x", "y"}, false},
		{"非法类型", 123, nil, true},
		{"非法 JSON", []byte(`not json`), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got StringArray
			err := got.Scan(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Scan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("Scan() len = %d, want %d", len(got), len(tt.want))
				}
				for i, v := range got {
					if i < len(tt.want) && v != tt.want[i] {
						t.Errorf("Scan()[%d] = %s, want %s", i, v, tt.want[i])
					}
				}
			}
		})
	}
}

// TestPost_TableName 测试表名映射。
func TestPost_TableName(t *testing.T) {
	p := Post{}
	if got := p.TableName(); got != "posts" {
		t.Errorf("TableName() = %s, want posts", got)
	}
}