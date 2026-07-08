package repo

import (
	"context"
	"fmt"
	"time"

	"go_projects/praProject1/cmd/task/model"
	"go_projects/praProject1/pkg/cache"
	"go_projects/praProject1/pkg/rdb"
)

// 任务缓存 TTL 配置
var taskTTL = cache.NewTTL(
	10*time.Minute, // 任务详情缓存 10 分钟
	30*time.Second,  // 空值哨兵 30 秒
)

// taskCacheClient 是任务缓存的全局客户端，由 InitTaskCache 初始化
var taskCacheClient *cache.Client

// InitTaskCache 初始化任务缓存层，在 main.go 中 Redis 初始化后调用
func InitTaskCache() {
	if rdb.RDB != nil {
		taskCacheClient = cache.New(rdb.RDB)
	}
}

// taskCacheKey 生成任务缓存 key，包含 school_id 实现多租户隔离
func taskCacheKey(schoolID, taskID int64) string {
	return fmt.Sprintf("task:school:%d:id:%d", schoolID, taskID)
}

// GetByIDWithCache 带缓存的任务查询（Cache-Aside 模式）
// 缓存命中直接返回；未命中查 DB 后回填缓存
func GetByIDWithCache(ctx context.Context, schoolID, taskID int64) (*model.Task, error) {
	if taskCacheClient == nil {
		return GetByID(schoolID, taskID)
	}

	key := taskCacheKey(schoolID, taskID)
	return cache.AsideGet(ctx, taskCacheClient, key, taskTTL, func() (*model.Task, error) {
		return GetByID(schoolID, taskID)
	})
}

// InvalidateTaskCache 删除任务缓存，在写操作后调用
func InvalidateTaskCache(ctx context.Context, schoolID, taskID int64) {
	if taskCacheClient == nil {
		return
	}
	key := taskCacheKey(schoolID, taskID)
	_ = cache.Delete(ctx, taskCacheClient, key)
}
