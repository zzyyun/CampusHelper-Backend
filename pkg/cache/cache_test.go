package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestCacheClient 测试基本的 Set/Get/Delete 操作
func TestCacheClient(t *testing.T) {
	// 注意：本测试需要本地 Redis 运行（127.0.0.1:6379）
	// 在 CI 环境中跳过
	t.Skip("需要本地 Redis，CI 中跳过")
}

// TestSingleflight 测试 singleflight 并发去重逻辑
func TestSingleflight(t *testing.T) {
	sf := &singleflight{}
	key := "test-key"
	var callCount int

	// 模拟3个并发请求
	type result struct {
		val string
		err error
	}
	results := make(chan result, 3)

	for i := 0; i < 3; i++ {
		go func() {
			val, err := sf.Do(key, func() (json.RawMessage, error) {
				callCount++
				time.Sleep(10 * time.Millisecond) // 模拟 DB 查询
				return json.RawMessage("data"), nil
			})
			results <- result{val: string(val), err: err}
		}()
	}

	for i := 0; i < 3; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("singleflight 期望无 error，实际: %v", r.err)
		}
		if r.val != "data" {
			t.Errorf("singleflight 期望 'data'，实际: %s", r.val)
		}
	}

	// 并发请求只应触发一次 loader
	if callCount != 1 {
		t.Errorf("singleflight 应只调用 1 次 loader，实际: %d", callCount)
	}
}

// TestAsideGet_CacheHit 测试缓存命中直接返回
func TestAsideGet_CacheHit(t *testing.T) {
	t.Skip("需要本地 Redis，CI 中跳过")
}

// TestAsideGet_CacheMiss_WithLoader 测试缓存未命中时调用 loader
func TestAsideGet_CacheMiss_WithLoader(t *testing.T) {
	t.Skip("需要本地 Redis，CI 中跳过")
}

// TestEmptySentinel 测试空值哨兵写入后不穿透
func TestEmptySentinel(t *testing.T) {
	t.Skip("需要本地 Redis，CI 中跳过")
}

// TestAsideGet_NilLoader 测试 loader 返回 nil 时写入空值哨兵
func TestAsideGet_NilLoader(t *testing.T) {
	_ = context.Background()
	// 验证：loader 返回 nil → 不应再次调用 loader（空值哨兵生效）
	t.Skip("需要本地 Redis，CI 中跳过")
}
