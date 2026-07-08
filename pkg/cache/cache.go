package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrCacheMiss 表示缓存未命中（调用方应穿透到 DB）
var ErrCacheMiss = errors.New("cache: miss")

// emptyValue 是缓存穿透防护的空值哨兵，存在 Redis 中标记"该 key 对应的 DB 记录不存在"
const emptyValue = "__CACHE_EMPTY__"

// Client 封装 Redis 客户端，提供 Cache-Aside 模式、穿透防护和击穿防护能力
type Client struct {
	rdb *redis.Client
	sg  singleflight // 防击穿：相同 key 的并发请求只放一个到 DB
}

// singleflight 合并相同 key 的并发请求，避免缓存击穿
type singleflight struct {
	mu sync.Mutex
	m  map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val json.RawMessage
	err error
}

func (sf *singleflight) Do(key string, fn func() (json.RawMessage, error)) (json.RawMessage, error) {
	sf.mu.Lock()
	if sf.m == nil {
		sf.m = make(map[string]*call)
	}
	if c, ok := sf.m[key]; ok {
		sf.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &call{}
	c.wg.Add(1)
	sf.m[key] = c
	sf.mu.Unlock()

	// panic 保护：fn panic 时仍要 Done + 清理 map，避免永久阻塞所有等待者
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.err = fmt.Errorf("singleflight: panic recovered: %v", r)
			}
			c.wg.Done()
			sf.mu.Lock()
			delete(sf.m, key)
			sf.mu.Unlock()
		}()
		c.val, c.err = fn()
	}()

	return c.val, c.err
}

// New 创建缓存客户端
func New(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Get 从缓存读取并反序列化到 dest。
// - 缓存命中且有值 → dest 有值，返回 nil
// - 缓存命中但为空哨兵 → 返回 ErrCacheMiss（穿透防护）
// - 缓存未命中 → 返回 ErrCacheMiss
func Get[T any](ctx context.Context, c *Client, key string, dest *T) error {
	val, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return fmt.Errorf("cache get %s: %w", key, err)
	}
	// 空值哨兵：穿透防护，DB 中不存在该记录
	if string(val) == emptyValue {
		return ErrCacheMiss
	}
	return json.Unmarshal(val, dest)
}

// Set 将值序列化后写入 Redis，支持自定义 TTL
func Set[T any](ctx context.Context, c *Client, key string, val *T, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("cache set %s: %w", key, err)
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

// SetEmpty 写入空值哨兵（穿透防护），TTL 较短避免长期占用内存
func SetEmpty(ctx context.Context, c *Client, key string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, emptyValue, ttl).Err()
}

// Delete 删除缓存（写操作后调用，失效策略）
func Delete(ctx context.Context, c *Client, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// AsideGet 执行 Cache-Aside 读模式：
// 1. 查缓存 → 命中直接返回
// 2. 未命中 → singleflight 防击穿 → 调用 loader 加载数据
// 3. loader 返回空数据 → 写入空值哨兵（穿透防护）
// 4. loader 返回有数据 → 写入缓存
func AsideGet[T any](ctx context.Context, c *Client, key string, ttl emptyTTL, loader func() (*T, error)) (*T, error) {
	// 1. 查缓存
	var cached T
	err := Get(ctx, c, key, &cached)
	if err == nil {
		return &cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err // Redis 真正的错误
	}

	// 2. 缓存未命中 → singleflight 防击穿（同一 key 并发只放一个请求到 DB）
	valRaw, err := c.sg.Do(key, func() (json.RawMessage, error) {
		data, loadErr := loader()
		if loadErr != nil {
			return nil, loadErr
		}
		if data == nil {
			// 3. 数据不存在 → 写空值哨兵
			_ = SetEmpty(ctx, c, key, ttl.empty)
			return nil, nil
		}
		// 4. 写入缓存
		b, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			return nil, marshalErr
		}
		_ = c.rdb.Set(ctx, key, b, ttl.data).Err()
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	if valRaw == nil {
		return nil, ErrCacheMiss
	}
	var result T
	if err := json.Unmarshal(valRaw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// emptyTTL 分别控制"有数据缓存"和"空值哨兵"的过期时间
type emptyTTL struct {
	data  time.Duration // 有数据时的 TTL
	empty time.Duration // 空值哨兵的 TTL（应短于 data，避免长时间占用）
}

// NewTTL 创建标准 TTL 配置
func NewTTL(data, empty time.Duration) emptyTTL {
	return emptyTTL{data: data, empty: empty}
}
