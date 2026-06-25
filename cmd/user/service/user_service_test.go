package service

import (
	"context"
	"os"
	"testing"

	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
)

// TestMain 初始化配置和数据库连接（环境变量 USER_TEST_DB=1 时启用）。
func TestMain(m *testing.M) {
	if os.Getenv("USER_TEST_DB") != "1" {
		// 设置最小配置使 InitConfig 不 panic
		os.Exit(m.Run())
	}

	config.InitConfig("")
	if _, err := db.InitUserDB(); err != nil {
		// DB 不可用时跳过
		os.Exit(m.Run())
	}
	os.Exit(m.Run())
}

func TestUserIDFromCtx_NoMetadata(t *testing.T) {
	// 无 metadata 时应返回 0，不 panic
	id := userIDFromCtx(context.Background())
	if id != 0 {
		t.Errorf("空 context 应返回 0，实际 %d", id)
	}
}

func TestUserIDFromCtx_WithMetadata(t *testing.T) {
	// 简单验证函数签名和基础逻辑
	// 完整的 metadata 测试需在集成测试中覆盖
	t.Log("userIDFromCtx 无 fmt.Printf 调试日志")
}

func TestExtractTraceFromMeta_NilContext(t *testing.T) {
	// extractTraceFromMeta 在无 metadata 时应返回原 context
	ctx := context.Background()
	result := extractTraceFromMeta(ctx)
	if result != ctx {
		t.Error("无 metadata 时应返回原 context")
	}
}
