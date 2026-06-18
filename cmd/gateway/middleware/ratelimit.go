package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go_projects/praProject1/config"
)

// ipBucket is a simple token-bucket per IP.
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

// RateLimit is a per-IP token-bucket rate limiter Gin middleware.
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
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		b.tokens--
		c.Next()
	}
}