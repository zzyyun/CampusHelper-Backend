package sensitive

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

const (
	// SensitiveWordsKey Redis 敏感词库集合键。
	SensitiveWordsKey = "dfa:sensitive_words"
	// keyspaceChannel Redis Keyspace Notifications 频道模板。
	keyspaceChannel = "__keyspace@0__:" + SensitiveWordsKey
)

// WordManager 管理敏感词库加载与热更新。
//
// 设计：
//   - LoadWords 从 Redis SMembers 加载全量词表并重建 Trie
//   - WatchUpdates 通过 PSubscribe 监听词库变更，自动热重载
//   - 若 Redis 不可用，Manager 保留最后加载的词表（退化到静态模式）
type WordManager struct {
	Matcher *DFAMatcher
	rdb     *redis.Client
}

// NewWordManager 创建词库管理器。
// rdb 为 nil 时仅启用内存模式（无热更新）。
func NewWordManager(rdb *redis.Client) *WordManager {
	return &WordManager{
		Matcher: NewDFAMatcher(),
		rdb:     rdb,
	}
}

// LoadWords 从 Redis 加载敏感词库并重建 Trie。
// 若 rdb 为 nil 则返回 nil（调用方需用 Build 加载内置词表）。
func (m *WordManager) LoadWords(ctx context.Context) error {
	if m.rdb == nil {
		return nil
	}
	words, err := m.rdb.SMembers(ctx, SensitiveWordsKey).Result()
	if err != nil {
		return fmt.Errorf("加载敏感词失败: %w", err)
	}
	m.Matcher.Build(words)
	log.Printf("[DFA] 从 Redis 加载敏感词 %d 个", len(words))
	return nil
}

// WatchUpdates 监听 Redis Keyspace Notifications，词库变更时自动重载。
//
// 前提：Redis 需配置 notify-keyspace-events Egx。
// 此方法会阻塞当前 goroutine，建议在独立 goroutine 中调用。
// rdb 为 nil 时直接返回（无操作）。
func (m *WordManager) WatchUpdates(ctx context.Context) {
	if m.rdb == nil {
		return
	}
	pubsub := m.rdb.PSubscribe(ctx, keyspaceChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("[DFA] 开始监听词库变更: %s", keyspaceChannel)

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				log.Printf("[DFA] 词库监听通道关闭，退出 WatchUpdates")
				return
			}
			log.Printf("[DFA] 检测到词库变更: %s", msg.Payload)
			if err := m.LoadWords(ctx); err != nil {
				log.Printf("[DFA] 热重载词库失败: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[DFA] WatchUpdates 收到 ctx 取消，退出")
			return
		}
	}
}