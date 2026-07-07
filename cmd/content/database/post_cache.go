package database

import (
	"context"
	"fmt"
	"time"

	content_db "go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/cmd/content/repo"
	"go_projects/praProject1/pkg/cache"
	"go_projects/praProject1/pkg/rdb"
)

// 缓存 TTL 配置
var (
	postTTL = cache.NewTTL(
		10*time.Minute, // 有数据缓存 10 分钟
		30*time.Second,  // 空值哨兵 30 秒（DB 中不存在的帖子，短时间内不再穿透）
	)
	postListTTL = cache.NewTTL(
		2*time.Minute, // 列表缓存 2 分钟（更新频率高，TTL 短）
		10*time.Second,
	)
)

// postCacheClient 是帖子缓存的全局客户端，由 InitPostCache 初始化
var postCacheClient *cache.Client

// InitPostCache 初始化帖子缓存层，在 main.go 中 Redis 初始化后调用
func InitPostCache() {
	if rdb.RDB != nil {
		postCacheClient = cache.New(rdb.RDB)
	}
}

// postCacheKey 生成帖子缓存 key，包含 school_id 实现多租户隔离
func postCacheKey(schoolID, postID int64) string {
	return fmt.Sprintf("post:school:%d:id:%d", schoolID, postID)
}

// GetByIDWithCache 带缓存的帖子查询（Cache-Aside 模式）
// 缓存命中直接返回；未命中查 DB 后回填缓存
// 空值哨兵防止不存在的 ID 被反复穿透到 DB
func GetByIDWithCache(ctx context.Context, schoolID, postID int64) (*content_db.Post, error) {
	if postCacheClient == nil {
		return repo.GetByID(schoolID, postID)
	}

	key := postCacheKey(schoolID, postID)
	return cache.AsideGet(ctx, postCacheClient, key, postTTL, func() (*content_db.Post, error) {
		return repo.GetByID(schoolID, postID)
	})
}

// InvalidatePostCache 删除帖子缓存，在写操作后调用
func InvalidatePostCache(ctx context.Context, schoolID, postID int64) {
	if postCacheClient == nil {
		return
	}
	key := postCacheKey(schoolID, postID)
	_ = cache.Delete(ctx, postCacheClient, key)
}

// InvalidatePostListCache 删除该学校下所有帖子列表缓存。
// 由于列表 key 含游标参数，这里使用 Redis SCAN 批量删除。
// 对于生产环境，建议后续改用 Redis Set 管理列表 key 方便批量失效。
func InvalidatePostListCache(ctx context.Context, schoolID int64) {
	if postCacheClient == nil {
		return
	}
	// 模式匹配删除该学校的帖子列表缓存
	pattern := fmt.Sprintf("postlist:school:%d:*", schoolID)
	iter := rdb.RDB.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		_ = cache.Delete(ctx, postCacheClient, keys...)
	}
}
