package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// testUser 用于序列化/反序列化的测试模型
type testUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// newTestClient 创建连接 miniredis 的缓存客户端
func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return New(rdb), mr
}

// ─── Get 单元测试 ──────────────────────────────────────────────────────────

// TestGet_CacheHit 验证：缓存中有有效值时，Get 正确反序列化并返回
func TestGet_CacheHit(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	// 预置数据到 Redis（miniredis.Set 不支持 TTL 参数，TTL 通过 SetTTL 单独设置）
	mr.Set("user:1", `{"id":1,"name":"张三"}`)
	mr.SetTTL("user:1", 10*time.Minute)

	var dest testUser
	err := Get(ctx, c, "user:1", &dest)
	if err != nil {
		t.Fatalf("期望无 error，实际: %v", err)
	}
	if dest.ID != 1 || dest.Name != "张三" {
		t.Fatalf("期望 {1, 张三}，实际: %+v", dest)
	}
}

// TestGet_CacheMiss 验证：key 不存在时返回 ErrCacheMiss
func TestGet_CacheMiss(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	var dest testUser
	err := Get(ctx, c, "user:999", &dest)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望 ErrCacheMiss，实际: %v", err)
	}
}

// TestGet_EmptySentinel 验证：命中空值哨兵时返回 ErrCacheMiss（穿透防护生效）
func TestGet_EmptySentinel(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	// 写入空值哨兵
	mr.Set("user:404", emptyValue)
	mr.SetTTL("user:404", 30*time.Second)

	var dest testUser
	err := Get(ctx, c, "user:404", &dest)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望 ErrCacheMiss（空值哨兵穿透防护），实际: %v", err)
	}
}

// ─── Set 单元测试 ──────────────────────────────────────────────────────────

// TestSet_Normal 验证：Set 写入后 Get 能正确读回
func TestSet_Normal(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	u := &testUser{ID: 42, Name: "李四"}
	err := Set(ctx, c, "user:42", u, 10*time.Minute)
	if err != nil {
		t.Fatalf("Set 期望无 error，实际: %v", err)
	}

	var dest testUser
	err = Get(ctx, c, "user:42", &dest)
	if err != nil {
		t.Fatalf("Get 期望无 error，实际: %v", err)
	}
	if dest.ID != 42 || dest.Name != "李四" {
		t.Fatalf("期望 {42, 李四}，实际: %+v", dest)
	}
}

// TestSet_TTLExpired 验证：写入后 TTL 过期，Get 返回 ErrCacheMiss
func TestSet_TTLExpired(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	u := &testUser{ID: 1, Name: "过期测试"}
	_ = Set(ctx, c, "user:exp", u, 10*time.Minute)

	// 手动推进 miniredis 时间
	mr.FastForward(11 * time.Minute)

	var dest testUser
	err := Get(ctx, c, "user:exp", &dest)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望 TTL 过期后返回 ErrCacheMiss，实际: %v", err)
	}
}

// ─── SetEmpty 单元测试 ─────────────────────────────────────────────────────

// TestSetEmpty_ThenGetMiss 验证：SetEmpty 后 Get 应返回 ErrCacheMiss
func TestSetEmpty_ThenGetMiss(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	err := SetEmpty(ctx, c, "user:null", 30*time.Second)
	if err != nil {
		t.Fatalf("SetEmpty 期望无 error，实际: %v", err)
	}

	var dest testUser
	err = Get(ctx, c, "user:null", &dest)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望空值哨兵返回 ErrCacheMiss，实际: %v", err)
	}
}

// TestSetEmpty_ValueIsSentinel 验证：SetEmpty 写入的值确实是空值哨兵常量
func TestSetEmpty_ValueIsSentinel(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	_ = SetEmpty(ctx, c, "check:empty", 30*time.Second)

	val, err := mr.Get("check:empty")
	if err != nil {
		t.Fatalf("期望能读取 key，实际: %v", err)
	}
	if val != emptyValue {
		t.Fatalf("期望值为 %q，实际: %q", emptyValue, val)
	}
}

// ─── Delete 单元测试 ────────────────────────────────────────────────────────

// TestDelete_ExistingKey 验证：删除已存在的 key 后 Get 返回 miss
func TestDelete_ExistingKey(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	mr.Set("user:del", `{"id":99,"name":"删除测试"}`)
	mr.SetTTL("user:del", 10*time.Minute)
	_ = Delete(ctx, c, "user:del")

	var dest testUser
	err := Get(ctx, c, "user:del", &dest)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望删除后返回 ErrCacheMiss，实际: %v", err)
	}
}

// TestDelete_EmptyKeys 验证：传入空 key 列表不报错
func TestDelete_EmptyKeys(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	err := Delete(ctx, c)
	if err != nil {
		t.Fatalf("期望空 key 列表不报错，实际: %v", err)
	}
}

// ─── AsideGet 核心逻辑测试 ─────────────────────────────────────────────────

// TestAsideGet_CacheHit 验证：缓存命中时直接返回，不调用 loader
func TestAsideGet_CacheHit(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	// 预置缓存
	mr.Set("post:1", `{"id":1,"name":"已缓存帖子"}`)
	mr.SetTTL("post:1", 10*time.Minute)

	ttl := NewTTL(10*time.Minute, 30*time.Second)
	loaderCalled := false
	loader := func() (*testUser, error) {
		loaderCalled = true
		return &testUser{ID: 999, Name: "loader返回的值"}, nil
	}

	result, err := AsideGet(ctx, c, "post:1", ttl, loader)
	if err != nil {
		t.Fatalf("期望无 error，实际: %v", err)
	}
	if result.ID != 1 || result.Name != "已缓存帖子" {
		t.Fatalf("期望返回缓存值 {1, 已缓存帖子}，实际: %+v", result)
	}
	if loaderCalled {
		t.Fatal("缓存命中时不应调用 loader")
	}
}

// TestAsideGet_CacheMiss_ThenLoaderCaches 验证：缓存未命中 → 调用 loader → 结果写入缓存
func TestAsideGet_CacheMiss_ThenLoaderCaches(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	ttl := NewTTL(10*time.Minute, 30*time.Second)
	loader := func() (*testUser, error) {
		return &testUser{ID: 10, Name: "loader查询结果"}, nil
	}

	// 第一次：缓存未命中，调用 loader
	result, err := AsideGet(ctx, c, "post:10", ttl, loader)
	if err != nil {
		t.Fatalf("期望无 error，实际: %v", err)
	}
	if result.ID != 10 || result.Name != "loader查询结果" {
		t.Fatalf("期望 loader 结果，实际: %+v", result)
	}

	// 验证：结果已写入缓存（二次读取直接命中，不调用 loader）
	loader2Called := false
	loader2 := func() (*testUser, error) {
		loader2Called = true
		return &testUser{ID: 999, Name: "不应出现"}, nil
	}
	result2, err := AsideGet(ctx, c, "post:10", ttl, loader2)
	if err != nil {
		t.Fatalf("第二次读取期望无 error，实际: %v", err)
	}
	if result2.ID != 10 {
		t.Fatalf("第二次读取期望缓存值，实际: %+v", result2)
	}
	if loader2Called {
		t.Fatal("第二次调用不应触发 loader（缓存已回填）")
	}
}

// TestAsideGet_NilLoaderWritesSentinel 验证：loader 返回 nil → 写入空值哨兵
func TestAsideGet_NilLoaderWritesSentinel(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	ttl := NewTTL(10*time.Minute, 30*time.Second)
	loader := func() (*testUser, error) {
		return nil, nil // DB 中不存在该记录
	}

	result, err := AsideGet(ctx, c, "post:null", ttl, loader)
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望 loader 返回 nil 时得到 ErrCacheMiss，实际: %v", err)
	}
	if result != nil {
		t.Fatalf("期望 result 为 nil，实际: %+v", result)
	}

	// 验证：空值哨兵已写入 Redis
	val, getErr := mr.Get("post:null")
	if getErr != nil {
		t.Fatalf("期望空值哨兵存在于 Redis，实际: %v", getErr)
	}
	if val != emptyValue {
		t.Fatalf("期望空值哨兵值 %q，实际: %q", emptyValue, val)
	}
}

// TestAsideGet_LoaderErrorPropagated 验证：loader 返回 error 时不写缓存，error 向上传播
func TestAsideGet_LoaderErrorPropagated(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	ttl := NewTTL(10*time.Minute, 30*time.Second)
	loaderErr := errors.New("database connection refused")
	loader := func() (*testUser, error) {
		return nil, loaderErr
	}

	_, err := AsideGet(ctx, c, "post:error", ttl, loader)
	if !errors.Is(err, loaderErr) {
		t.Fatalf("期望原始 error 传播，实际: %v", err)
	}

	// 验证：Redis 中没有该 key（loader 出错不写缓存）
	if mr.Exists("post:error") {
		t.Fatal("loader 出错时不应写入缓存")
	}
}

// TestAsideGet_SingleflightDedup 验证：并发请求同一 key，loader 只调用一次
func TestAsideGet_SingleflightDedup(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	ttl := NewTTL(10*time.Minute, 30*time.Second)
	var loaderCallCount int64
	loader := func() (*testUser, error) {
		atomic.AddInt64(&loaderCallCount, 1)
		time.Sleep(20 * time.Millisecond) // 模拟慢 DB 查询
		return &testUser{ID: 1, Name: "并发测试"}, nil
	}

	// 启动10个并发请求
	type result struct {
		user *testUser
		err  error
	}
	ch := make(chan result, 10)
	for i := 0; i < 10; i++ {
		go func() {
			u, e := AsideGet(ctx, c, "post:concurrent", ttl, loader)
			ch <- result{u, e}
		}()
	}

	// 收集结果
	for i := 0; i < 10; i++ {
		r := <-ch
		if r.err != nil {
			t.Errorf("goroutine %d: 期望无 error，实际: %v", i, r.err)
		}
		if r.user == nil || r.user.ID != 1 {
			t.Errorf("goroutine %d: 期望 {1, 并发测试}，实际: %+v", i, r.user)
		}
	}

	// 关键断言：10 个并发请求只触发 1 次 loader
	count := atomic.LoadInt64(&loaderCallCount)
	if count != 1 {
		t.Fatalf("期望 singleflight 合并后只调用 1 次 loader，实际: %d 次", count)
	}
}

// ─── singleflight 边界测试 ──────────────────────────────────────────────────

// TestSingleflight_ErrorPropagated 验证：loader 返回 error 时，所有等待者都收到同一 error
func TestSingleflight_ErrorPropagated(t *testing.T) {
	sf := &singleflight{}
	key := "error-key"
	expectedErr := errors.New("DB down")
	var callCount int64

	type result struct {
		val string
		err error
	}
	ch := make(chan result, 5)
	for i := 0; i < 5; i++ {
		go func() {
			val, err := sf.Do(key, func() (json.RawMessage, error) {
				atomic.AddInt64(&callCount, 1)
				time.Sleep(10 * time.Millisecond)
				return nil, expectedErr
			})
			ch <- result{val: string(val), err: err}
		}()
	}

	for i := 0; i < 5; i++ {
		r := <-ch
		if !errors.Is(r.err, expectedErr) {
			t.Errorf("goroutine %d: 期望 error %v，实际: %v", i, expectedErr, r.err)
		}
	}

	if atomic.LoadInt64(&callCount) != 1 {
		t.Fatalf("期望只调用 1 次 loader，实际: %d", atomic.LoadInt64(&callCount))
	}
}

// TestSingleflight_DifferentKeys 验证：不同 key 的请求互不影响
func TestSingleflight_DifferentKeys(t *testing.T) {
	sf := &singleflight{}
	var callCountA, callCountB int64

	doneA := make(chan struct{})
	doneB := make(chan struct{})

	// key A 的请求（慢）
	go func() {
		sf.Do("key-a", func() (json.RawMessage, error) {
			atomic.AddInt64(&callCountA, 1)
			time.Sleep(50 * time.Millisecond)
			return json.RawMessage("a"), nil
		})
		close(doneA)
	}()

	// key B 的请求（快，应独立执行）
	go func() {
		sf.Do("key-b", func() (json.RawMessage, error) {
			atomic.AddInt64(&callCountB, 1)
			return json.RawMessage("b"), nil
		})
		close(doneB)
	}()

	<-doneA
	<-doneB

	if atomic.LoadInt64(&callCountA) != 1 {
		t.Errorf("key-a 应调用 1 次，实际: %d", atomic.LoadInt64(&callCountA))
	}
	if atomic.LoadInt64(&callCountB) != 1 {
		t.Errorf("key-b 应调用 1 次，实际: %d", atomic.LoadInt64(&callCountB))
	}
}
