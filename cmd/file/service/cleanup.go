package service

import (
	"log"
	"time"

	"go_projects/praProject1/cmd/file/repo"
)

// StartCleanupTask 启动后台任务，每天清理一次 30 天前已软删除的文件。
func StartCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动时立即清理一次
	doCleanup()

	for range ticker.C {
		doCleanup()
	}
}

// doCleanup 物理删除超过 30 天已软删除的文件。
func doCleanup() {
	before := time.Now().Add(-30 * 24 * time.Hour)
	count, err := repo.CleanupBefore(before)
	if err != nil {
		log.Printf("[file-service] 清理文件失败: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[file-service] 已清理 %d 个 30 天前已删除的文件", count)
	}
}