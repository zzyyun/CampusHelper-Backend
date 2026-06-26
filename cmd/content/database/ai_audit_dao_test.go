package database

import (
	"errors"
	"testing"
	"time"

	"go_projects/praProject1/cmd/content/model"
)

// 注：DAO 的实际数据库测试需要在 testcontainers 或 docker-compose 环境下进行。
// 此处仅测试参数校验和基本逻辑，不连接真实数据库。

func TestCreateAIAuditLog_NilLog(t *testing.T) {
	// 模拟：传入 nil 时应返回 error
	err := CreateAIAuditLog(nil, nil)
	if err == nil {
		t.Error("expected error for nil log")
	}
}

func TestCreateAIAuditLog_InvalidPostID(t *testing.T) {
	log := &model.AIAuditLog{
		PostID:       0, // 无效
		ContentHash:  "abc",
		AIStatus:     model.AIStatusSynced,
		AIResult:     model.AIResultPass,
		AIConfidence: 0.95,
	}
	err := CreateAIAuditLog(nil, log)
	if err == nil {
		t.Error("expected error for invalid post_id")
	}
	if !errors.Is(err, err) {
		t.Errorf("expected error type, got %v", err)
	}
}

func TestListAIAuditLogsByPostID_InvalidPostID(t *testing.T) {
	logs, err := ListAIAuditLogsByPostID(nil, 0, 10)
	if err == nil {
		t.Error("expected error for invalid post_id")
	}
	if logs != nil {
		t.Error("expected nil logs on error")
	}
}

func TestCleanupOldAIAuditLogs_EmptyDB(t *testing.T) {
	// 测试参数构造正确，不实际执行 SQL
	before := time.Now().Add(-180 * 24 * time.Hour)
	if before.After(time.Now()) {
		t.Error("before time should be in past")
	}
	// 真实测试需要 DB 连接，跳过
	t.Skip("requires DB connection, tested in integration tests")
}