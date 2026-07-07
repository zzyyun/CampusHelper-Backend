package middleware

import (
	"fmt"
	"log"
	"sync"
	"time"

	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// ipBucket 单 IP 的令牌桶状态。
type ipBucket struct {
	tokens   float64
	lastSeen time.Time
	mu       sync.Mutex
}

var (
	buckets   = make(map[string]*ipBucket)
	bucketsMu sync.Mutex
)

func getBucket(ip string) *ipBucket {
	bucketsMu.Lock()
	defer bucketsMu.Unlock()
	b, ok := buckets[ip]
	if !ok {
		b = &ipBucket{tokens: float64(config.Conf.Gateway.RateBurst), lastSeen: time.Now()}
		buckets[ip] = b
	}
	return b
}

// cleanupStaleBuckets 定期清理超过 10 分钟未访问的 IP 桶，防止内存泄漏
func cleanupStaleBuckets() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		bucketsMu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		cleaned := 0
		for ip, b := range buckets {
			b.mu.Lock()
			stale := b.lastSeen.Before(cutoff)
			b.mu.Unlock()
			if stale {
				delete(buckets, ip)
				cleaned++
			}
		}
		bucketsMu.Unlock()
		if cleaned > 0 {
			log.Printf("[ratelimit] 清理过期桶: %d 个 IP 已移除", cleaned)
		}
	}
}

func init() {
	go cleanupStaleBuckets()
}

// RateLimit 基于令牌桶的 IP 级别限流，超限返回 HTTP 429 + Retry-After 头。
func RateLimit() gin.HandlerFunc {
	rate := config.Conf.Gateway.RateLimit
	burst := float64(config.Conf.Gateway.RateBurst)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		b := getBucket(ip)
		b.mu.Lock()
		defer b.mu.Unlock()

		now := time.Now()
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * rate
		if b.tokens > burst {
			b.tokens = burst
		}
		b.lastSeen = now

		if b.tokens < 1 {
			// 计算需要等待多少秒才能获得一个令牌
			retryAfter := int((1 - b.tokens) / rate)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			ErrorResponse(c, errcode.ErrRateLimited, "请求过于频繁，请稍后再试")
			return
		}
		b.tokens--
		c.Next()
	}
}