package middleware

import (
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// circuitState 熔断器状态
type circuitState int

const (
	circuitClosed   circuitState = iota // 正常（放行所有请求）
	circuitOpen                         // 熔断（拒绝所有请求，返回降级响应）
	circuitHalfOpen                     // 半开（放行少量探测请求）
)

// serviceCircuit 单个服务的熔断器状态
type serviceCircuit struct {
	state           circuitState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	mu              sync.Mutex
}

// 降级响应：服务暂时不可用，前端可展示缓存数据或友好提示
const degradationMessage = "服务暂时繁忙，请稍后重试"

var (
	// 按服务名隔离的熔断器
	circuits   = make(map[string]*serviceCircuit)
	circuitsMu sync.RWMutex

	// 配置项（可通过 config 覆盖）
	circuitFailureThreshold = 5                // 连续失败次数触发熔断
	circuitRecoveryTimeout  = 30 * time.Second // 熔断恢复窗口（30s 后进入半开）
	circuitSuccessThreshold = 3                // 半开状态下连续成功次数恢复
)

// getCircuit 获取服务对应的熔断器
func getCircuit(service string) *serviceCircuit {
	circuitsMu.RLock()
	c, ok := circuits[service]
	circuitsMu.RUnlock()
	if ok {
		return c
	}
	circuitsMu.Lock()
	defer circuitsMu.Unlock()
	// 双重检查
	if c, ok = circuits[service]; ok {
		return c
	}
	c = &serviceCircuit{state: circuitClosed}
	circuits[service] = c
	return c
}

// RecordFailure 记录一次服务调用失败，触发熔断判定
func RecordFailure(service string) {
	c := getCircuit(service)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()

	if c.state == circuitHalfOpen {
		// 半开状态下失败 → 重新熔断
		c.state = circuitOpen
		log.Printf("[degradation] 服务 %s 熔断器打开（半开态探测失败）", service)
	} else if c.failureCount >= circuitFailureThreshold && c.state == circuitClosed {
		c.state = circuitOpen
		log.Printf("[degradation] 服务 %s 熔断器打开（连续失败 %d 次）", service, c.failureCount)
	}
}

// RecordSuccess 记录一次服务调用成功
func RecordSuccess(service string) {
	c := getCircuit(service)
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case circuitHalfOpen:
		c.successCount++
		if c.successCount >= circuitSuccessThreshold {
			c.state = circuitClosed
			c.failureCount = 0
			c.successCount = 0
			log.Printf("[degradation] 服务 %s 熔断器恢复（半开态连续成功 %d 次）", service, circuitSuccessThreshold)
		}
	case circuitClosed:
		c.failureCount = 0 // 正常态下成功清零失败计数
	}
}

// IsDegraded 检查指定服务是否处于降级状态
func IsDegraded(service string) bool {
	c := getCircuit(service)
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == circuitOpen {
		// 检查是否过了恢复窗口
		if time.Since(c.lastFailureTime) > circuitRecoveryTimeout {
			c.state = circuitHalfOpen
			c.successCount = 0
			log.Printf("[degradation] 服务 %s 进入半开态（%v 窗口到期）", service, circuitRecoveryTimeout)
			return false
		}
		return true
	}
	return false
}

// DegradationMiddleware 降级中间件（按服务粒度控制）
// 当指定服务的熔断器打开时，返回 503 + 降级提示
// 用法：r.Use(middleware.DegradationMiddleware("content", "task"))
func DegradationMiddleware(services ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, svc := range services {
			if IsDegraded(svc) {
				c.Header("Content-Type", "application/json")
				c.Header("X-Degraded-Service", svc)
				c.AbortWithStatusJSON(503, gin.H{
					"code":    50300,
					"message": degradationMessage,
					"service": svc,
				})
				return
			}
		}
		c.Next()
	}
}

// ResetCircuit 重置指定服务的熔断器（供管理接口或测试调用）
func ResetCircuit(service string) {
	c := getCircuit(service)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = circuitClosed
	c.failureCount = 0
	c.successCount = 0
	log.Printf("[degradation] 服务 %s 熔断器已重置", service)
}
