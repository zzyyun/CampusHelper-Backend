// Package ai_moderation - config_manager.go 提供运行时配置热更新能力。
//
// 支持：
//   - 阈值动态调整（block_threshold / review_threshold）
//   - 模型优雅重载（替换 ModelLoader，旧 loader 自动 Close）
//   - 线程安全（sync.RWMutex 保护所有配置读写）
package ai_moderation

import (
	"log"
	"sync"

	"go_projects/praProject1/internal/ai_moderation/types"
)

// ThresholdConfig 审核阈值配置。
type ThresholdConfig struct {
	BlockThreshold  float32 // ≥ 此值 → BLOCK（默认 0.9）
	ReviewThreshold float32 // ≥ 此值 → REVIEW（默认 0.5）
}

// DefaultThresholdConfig 默认阈值。
func DefaultThresholdConfig() ThresholdConfig {
	return ThresholdConfig{
		BlockThreshold:  0.9,
		ReviewThreshold: 0.5,
	}
}

// ConfigManager 运行时配置管理器（线程安全）。
type ConfigManager struct {
	mu          sync.RWMutex
	thresholds  ThresholdConfig
	loader      types.ModelLoader
	reloadCount int // 累计重载次数
}

// NewConfigManager 创建配置管理器。
func NewConfigManager(loader types.ModelLoader, thresholds ThresholdConfig) *ConfigManager {
	return &ConfigManager{
		thresholds: thresholds,
		loader:     loader,
	}
}

// GetThresholds 获取当前阈值（线程安全）。
func (m *ConfigManager) GetThresholds() ThresholdConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.thresholds
}

// UpdateThresholds 更新阈值配置（线程安全）。
//
// 校验规则：review_threshold < block_threshold。
func (m *ConfigManager) UpdateThresholds(block, review float32) error {
	if review >= block {
		return ErrInvalidThreshold
	}
	if block <= 0 || block > 1 || review < 0 || review > 1 {
		return ErrInvalidThreshold
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.thresholds = ThresholdConfig{
		BlockThreshold:  block,
		ReviewThreshold: review,
	}
	log.Printf("[config] thresholds updated: block=%.2f, review=%.2f", block, review)
	return nil
}

// GetLoader 获取当前 loader。
func (m *ConfigManager) GetLoader() types.ModelLoader {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loader
}

// ReloadLoader 优雅重载模型 loader。
//
// 流程：创建新 loader → 原子替换 → 关闭旧 loader。
func (m *ConfigManager) ReloadLoader(newLoader types.ModelLoader) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldLoader := m.loader
	m.loader = newLoader
	m.reloadCount++

	log.Printf("[config] loader reloaded (count=%d)", m.reloadCount)

	// 关闭旧 loader
	if oldLoader != nil {
		if err := oldLoader.Close(); err != nil {
			log.Printf("[config] WARN: old loader close error: %v", err)
		}
	}

	return nil
}

// ReloadCount 返回累计重载次数。
func (m *ConfigManager) ReloadCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reloadCount
}

// Decide 根据当前阈值和概率做出决策。
//
// 阈值语义（PRD § Feature 1）：
//   - prob ≥ blockThreshold → BLOCK
//   - prob ≥ reviewThreshold → REVIEW
//   - otherwise → PASS
func (m *ConfigManager) Decide(prob float32) types.Result {
	m.mu.RLock()
	thresholds := m.thresholds
	m.mu.RUnlock()

	if prob >= thresholds.BlockThreshold {
		return types.ResultBlock
	}
	if prob >= thresholds.ReviewThreshold {
		return types.ResultReview
	}
	return types.ResultPass
}

// ─── 错误定义 ───────────────────────────────────────────────────────────────

// ErrInvalidThreshold 阈值配置无效。
var ErrInvalidThreshold = &ThresholdError{"invalid threshold: review must be < block, all in [0,1]"}

// ThresholdError 阈值配置错误。
type ThresholdError struct {
	msg string
}

func (e *ThresholdError) Error() string {
	return e.msg
}
