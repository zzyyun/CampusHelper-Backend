package db

import (
	"testing"

	"gorm.io/gorm"
)

// fakeDB 创建一个不需要 sqlite 的内存 gorm.DB。
// 实际只用来调用 Scope 函数验证其返回值。
// 这里通过创建一个最小化的 gorm.DB 来应用 Scope。
func fakeDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 使用 session 模式创建一个无需连接的 DB 实例
	// gorm v1.25 支持 session 模式进行 dry-run
	return &gorm.DB{}
}

// TestSchoolScope_ReturnsFunc 测试 Scope 函数返回类型。
func TestSchoolScope_ReturnsFunc(t *testing.T) {
	scope := SchoolScope(100)
	if scope == nil {
		t.Fatal("SchoolScope returned nil")
	}
}

// TestSchoolScope_ZeroSchoolID 测试 schoolID <= 0 时返回拦截条件。
func TestSchoolScope_ZeroSchoolID(t *testing.T) {
	// 仅验证函数能正确返回（不实际执行 SQL）
	scope := SchoolScope(0)
	if scope == nil {
		t.Fatal("SchoolScope(0) returned nil")
	}
	scope2 := SchoolScope(-1)
	if scope2 == nil {
		t.Fatal("SchoolScope(-1) returned nil")
	}
}

// TestSchoolIDFromContext 测试 context 提取工具函数。
func TestSchoolIDFromContext(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want int64
	}{
		{"正常 schoolID", 100, 100},
		{"schoolID=0 返回 0", 0, 0},
		{"schoolID=-1 拦截", -1, 0},
		{"schoolID=-100 拦截", -100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SchoolIDFromContext(tt.in); got != tt.want {
				t.Errorf("SchoolIDFromContext(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}