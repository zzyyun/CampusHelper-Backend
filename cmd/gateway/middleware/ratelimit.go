package middleware

import (
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

// RateLimit 基于令牌桶的 IP 级别限流。
//
// 超限时统一返回 30001 rate limit exceeded。
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
			ErrorResponse(c, errcode.ErrRateLimited, "请求过于频繁，请稍后再试")
			return
		}
		b.tokens--
		c.Next()
	}
}