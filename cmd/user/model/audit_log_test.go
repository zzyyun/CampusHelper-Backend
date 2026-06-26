package model

import "testing"

func TestAuditAction_Values(t *testing.T) {
	if AuditActionBanUser != "ban_user" {
		t.Errorf("AuditActionBanUser 应为 ban_user，实际 %s", AuditActionBanUser)
	}
	if AuditActionUnbanUser != "unban_user" {
		t.Errorf("AuditActionUnbanUser 应为 unban_user，实际 %s", AuditActionUnbanUser)
	}
	if AuditActionSetRole != "set_role" {
		t.Errorf("AuditActionSetRole 应为 set_role，实际 %s", AuditActionSetRole)
	}
	if AuditActionAuditPost != "audit_content" {
		t.Errorf("AuditActionAuditPost 应为 audit_content，实际 %s", AuditActionAuditPost)
	}
}

func TestAdminAuditLog_TableName(t *testing.T) {
	log := AdminAuditLog{}
	if log.TableName() != "admin_audit_logs" {
		t.Errorf("AdminAuditLog.TableName() 应为 admin_audit_logs，实际 %s", log.TableName())
	}
}
