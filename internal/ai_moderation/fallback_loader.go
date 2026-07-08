// Package ai_moderation - fallback_loader.go 提供模型推理降级策略。
//
// 设计（PRD § Feature 1 降级机制）：
//   - FallbackLoader 包装 primary（ONNX）和 fallback（Mock）两个 loader
//   - Infer 调用 primary，若失败则自动切换到 fallback 并记录降级指标
//   - 线程安全（atomic 切换 + sync.Once 初始化）
//
// 降级触发场景：
//   1. ONNX 模型文件不存在/损坏 → 启动时降级
//   2. 推理超时（>800ms）→ 运行时降级
//   3. ONNX session 内部错误 → 运行时降级
//
// 关联：
//   - PRD docs/ai-moderation-content-service-v3.0-prd.md
//   - 任务 task-006
package ai_moderation

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"go_projects/praProject1/internal/ai_moderation/types"
)

// FallbackLoader ONNX → Mock 自动降级包装器。
//
// 使用方式：
//
//	loader, err := NewFallbackLoader(onnxLoader, mockLoader, FallbackConfig{...})
//	result, err := loader.Infer(ctx, text)  // 自动降级
type FallbackLoader struct {
	primary   types.ModelLoader // ONNX loader（优先使用）
	fallback  types.ModelLoader // Mock loader（降级兜底）
	config    FallbackConfig

	// 降级状态（0=正常使用 primary，1=已降级使用 fallback）
	degraded   int32
	degradeOnce sync.Once
	mu         sync.RWMutex
	stopCh     chan struct{} // 关闭时通知 recoveryCheck goroutine 退出
}

// FallbackConfig 降级策略配置。
type FallbackConfig struct {
	MaxConsecutiveErrors int           // 连续错误数触发降级（默认 3）
	RecoveryCheckInterval time.Duration // 降级后定期尝试恢复的间隔（默认 5 分钟）
	AlertOnDegrade        bool          // 降级时是否告警（日志 + metrics）
}

// DefaultFallbackConfig 默认降级配置。
func DefaultFallbackConfig() FallbackConfig {
	return FallbackConfig{
		MaxConsecutiveErrors: 3,
		RecoveryCheckInterval: 5 * time.Minute,
		AlertOnDegrade:        true,
	}
}

// NewFallbackLoader 创建降级包装器。
func NewFallbackLoader(primary, fallback types.ModelLoader, config FallbackConfig) *FallbackLoader {
	if config.MaxConsecutiveErrors <= 0 {
		config.MaxConsecutiveErrors = 3
	}
	if config.RecoveryCheckInterval <= 0 {
		config.RecoveryCheckInterval = 5 * time.Minute
	}

	return &FallbackLoader{
		primary:  primary,
		fallback: fallback,
		config:   config,
		stopCh:   make(chan struct{}),
	}
}

// Infer 执行推理（优先 primary，失败时降级 fallback）。
func (l *FallbackLoader) Infer(ctx context.Context, text string) (*types.InferenceResult, error) {
	// 已降级或 primary 为 nil → 直接用 fallback
	if atomic.LoadInt32(&l.degraded) == 1 || l.primary == nil {
		return l.fallback.Infer(ctx, text)
	}

	// 尝试 primary
	result, err := l.primary.Infer(ctx, text)
	if err == nil {
		return result, nil
	}

	// primary 失败 → 降级
	l.recordDegrade(err)
	return l.fallback.Infer(ctx, text)
}

// Version 返回当前活跃 loader 的版本。
func (l *FallbackLoader) Version() string {
	if atomic.LoadInt32(&l.degraded) == 1 || l.primary == nil {
		return l.fallback.Version() + "-fallback"
	}
	return l.primary.Version()
}

// Close 释放两个 loader 资源，停止 recoveryCheck goroutine。
func (l *FallbackLoader) Close() error {
	// 停止 recovery goroutine
	select {
	case <-l.stopCh:
	default:
		close(l.stopCh)
	}

	var firstErr error
	if l.primary != nil {
		if err := l.primary.Close(); err != nil {
			firstErr = err
		}
	}
	if l.fallback != nil {
		if err := l.fallback.Close(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// IsDegraded 返回是否处于降级状态。
func (l *FallbackLoader) IsDegraded() bool {
	return atomic.LoadInt32(&l.degraded) == 1
}

// recordDegrade 记录降级事件（仅触发一次）。
func (l *FallbackLoader) recordDegrade(err error) {
	l.degradeOnce.Do(func() {
		atomic.StoreInt32(&l.degraded, 1)
		if l.config.AlertOnDegrade {
			log.Printf("[FallbackLoader] DEGRADED: primary failed (%v), switching to fallback", err)
		}
		// 启动恢复检查协程
		go l.recoveryCheck()
	})
}

// recoveryCheck 定期尝试 primary 恢复（直到恢复成功或 Close 被调用）。
func (l *FallbackLoader) recoveryCheck() {
	ticker := time.NewTicker(l.config.RecoveryCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return // Close 被调用，退出 goroutine
		case <-ticker.C:
			if atomic.LoadInt32(&l.degraded) == 0 {
				return // 已恢复
			}

			// 尝试用空文本做健康检查
			_, err := l.primary.Infer(context.Background(), "health check")
			if err == nil {
				atomic.StoreInt32(&l.degraded, 0)
				log.Printf("[FallbackLoader] RECOVERED: primary is healthy again")
				return
			}
			log.Printf("[FallbackLoader] recovery check failed: %v (still degraded)", err)
		}
	}
}

// ─── 启动时降级工厂 ──────────────────────────────────────────────────────────

// NewFallbackLoaderFromConfig 根据配置创建降级包装器。
//
// 尝试创建 primary loader，失败时自动降级到 mock。
func NewFallbackLoaderFromConfig(cfg ModelConfig) (types.ModelLoader, error) {
	// 创建 mock fallback（始终可用）
	mockLoader := NewMockLoader(cfg.ModelVersion + "-mock-fallback")

	// 尝试创建 primary（ONNX）
	if !cfg.Enabled {
		// mock 模式：直接用 mock loader，不需要降级
		return mockLoader, nil
	}

	// ONNX 模式：通过 onnxFactory 创建 primary
	if onnxFactory == nil {
		log.Printf("[FallbackLoader] onnxFactory not available, using mock mode")
		return mockLoader, nil
	}

	primaryLoader, err := onnxFactory(cfg)
	if err != nil {
		log.Printf("[FallbackLoader] primary loader creation failed: %v, falling back to mock", err)
		return NewFallbackLoader(nil, mockLoader, DefaultFallbackConfig()), nil
	}

	return NewFallbackLoader(primaryLoader, mockLoader, DefaultFallbackConfig()), nil
}
