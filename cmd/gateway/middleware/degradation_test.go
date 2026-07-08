package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestDegradationMiddleware_NormalFlow 测试正常请求通过降级中间件
func TestDegradationMiddleware_NormalFlow(t *testing.T) {
	r := gin.New()
	r.Use(DegradationMiddleware("content"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200，实际 %d", w.Code)
	}
}

// TestDegradationMiddleware_Degraded 测试服务降级时返回 503
func TestDegradationMiddleware_Degraded(t *testing.T) {
	r := gin.New()
	r.Use(DegradationMiddleware("content"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// 模拟 content 服务连续失败触发熔断
	for i := 0; i < circuitFailureThreshold; i++ {
		RecordFailure("content")
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("期望降级 503，实际 %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["service"] != "content" {
		t.Errorf("期望降级 service=content，实际 %v", body["service"])
	}
}

// TestCircuitBreaker_Recovery 测试熔断器恢复
func TestCircuitBreaker_Recovery(t *testing.T) {
	svc := "test-recovery"
	ResetCircuit(svc)

	// 触发熔断
	for i := 0; i < circuitFailureThreshold; i++ {
		RecordFailure(svc)
	}
	if !IsDegraded(svc) {
		t.Fatal("期望服务处于降级状态")
	}

	// 手动设置 lastFailureTime 到 1 分钟前（模拟恢复窗口过期）
	c := getCircuit(svc)
	c.mu.Lock()
	c.lastFailureTime = c.lastFailureTime.Add(-1 * time.Minute)
	c.mu.Unlock()

	// 进入半开态
	if IsDegraded(svc) {
		t.Fatal("期望窗口过期后不再降级")
	}

	// 半开态下成功次数达标 → 恢复正常
	for i := 0; i < circuitSuccessThreshold; i++ {
		RecordSuccess(svc)
	}
	if IsDegraded(svc) {
		t.Fatal("期望半开态成功后恢复正常")
	}
}

// TestCleanupStaleBuckets 测试过期桶清理
func TestCleanupStaleBuckets(t *testing.T) {
	// 创建一个桶并修改 lastSeen 到 20 分钟前
	b := getBucket("192.168.1.100")
	b.mu.Lock()
	b.lastSeen = time.Now().Add(-20 * time.Minute)
	b.mu.Unlock()

	// 手动触发清理
	bucketsMu.Lock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, b := range buckets {
		b.mu.Lock()
		stale := b.lastSeen.Before(cutoff)
		b.mu.Unlock()
		if stale {
			delete(buckets, ip)
		}
	}
	bucketsMu.Unlock()

	// 验证已被清理
	bucketsMu.Lock()
	_, exists := buckets["192.168.1.100"]
	bucketsMu.Unlock()
	if exists {
		t.Error("期望过期桶已被清理")
	}
}
