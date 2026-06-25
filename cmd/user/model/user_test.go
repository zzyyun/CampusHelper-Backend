package model

import "testing"

func TestRole_String(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleStudent, "student"},
		{RoleAdmin, "admin"},
		{RoleSuperAdmin, "super_admin"},
		{Role(0), "student"},  // 默认
		{Role(99), "student"}, // 未知
	}
	for _, tc := range tests {
		got := tc.role.String()
		if got != tc.want {
			t.Errorf("Role(%d).String() = %q，期望 %q", tc.role, got, tc.want)
		}
	}
}

func TestCan_Permissions(t *testing.T) {
	tests := []struct {
		name  string
		role  Role
		perm  Permission
		want  bool
	}{
		{"学生可发帖", RoleStudent, PermPostCreate, true},
		{"学生可创建任务", RoleStudent, PermTaskCreate, true},
		{"学生不可审核", RoleStudent, PermContentAudit, false},
		{"学生不可封禁", RoleStudent, PermUserBan, false},
		{"管理员可审核", RoleAdmin, PermContentAudit, true},
		{"管理员可封禁", RoleAdmin, PermUserBan, true},
		{"管理员不可管理用户", RoleAdmin, PermUserManage, false},
		{"超级管理员可管理用户", RoleSuperAdmin, PermUserManage, true},
		{"未知权限", RoleStudent, Permission("unknown"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Can(tc.role, tc.perm)
			if got != tc.want {
				t.Errorf("Can(%v, %v) = %v，期望 %v", tc.role, tc.perm, got, tc.want)
			}
		})
	}
}

func TestUserStatus_Values(t *testing.T) {
	if StatusNormal != 1 {
		t.Errorf("StatusNormal 应为 1，实际 %d", StatusNormal)
	}
	if StatusBanned != 2 {
		t.Errorf("StatusBanned 应为 2，实际 %d", StatusBanned)
	}
	if StatusDeleted != 3 {
		t.Errorf("StatusDeleted 应为 3，实际 %d", StatusDeleted)
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleStudent != 1 {
		t.Errorf("RoleStudent 应为 1，实际 %d", RoleStudent)
	}
	if RoleAdmin != 2 {
		t.Errorf("RoleAdmin 应为 2，实际 %d", RoleAdmin)
	}
	if RoleSuperAdmin != 3 {
		t.Errorf("RoleSuperAdmin 应为 3，实际 %d", RoleSuperAdmin)
	}
}
