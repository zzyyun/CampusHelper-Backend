package service

import (
	"log"
	"time"

	user_database "go_projects/praProject1/cmd/user/database"
)

// StartAuditCleanupTask 启动后台任务，每天清理一次 90 天前的审计日志。
func StartAuditCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动时立即清理一次
	doAuditCleanup()

	for range ticker.C {
		doAuditCleanup()
	}
}

func doAuditCleanup() {
	before := time.Now().Add(-90 * 24 * time.Hour)
	count, err := user_database.CleanupAuditLogs(before)
	if err != nil {
		log.Printf("[user-service] 清理审计日志失败: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[user-service] 已清理 %d 条 90 天前的审计日志", count)
	}
}
