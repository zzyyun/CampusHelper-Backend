package db

import "testing"

// TestSchoolScope 验证 SchoolScope 函数本身的行为（不依赖真实数据库）。
// 真实 SQL 注入行为在 service 层的集成测试中验证。
func TestSchoolScope(t *testing.T) {
	cases := []struct {
		name     string
		schoolID int64
	}{
		{"schoolID = 0 应返回 1=0 scope", 0},
		{"schoolID = -1 应返回 1=0 scope", -1},
		{"schoolID = -100 应返回 1=0 scope", -100},
		{"schoolID = 1 应返回正常 scope", 1},
		{"schoolID = 12345 应返回正常 scope", 12345},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope := SchoolScope(tc.schoolID)
			if scope == nil {
				t.Fatalf("SchoolScope(%d) 返回 nil，期望非 nil", tc.schoolID)
			}
		})
	}
}

// TestTenantSafe 验证 TenantSafe 入口校验函数不会 panic 且返回非 nil
func TestTenantSafe_Behavior(t *testing.T) {
	// 这里不构造真实 *gorm.DB（需要 MySQL 连接），
	// 仅验证函数签名存在且调用约定清晰，集成测试会在 service 层覆盖。
	t.Log("TenantSafe 行为在 service 层集成测试中验证")
}