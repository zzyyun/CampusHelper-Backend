package ai_moderation

import (
	"sync"
	"testing"

	"go_projects/praProject1/internal/ai_moderation/types"
)

func TestConfigManager_DefaultThresholds(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	mgr := NewConfigManager(loader, DefaultThresholdConfig())

	th := mgr.GetThresholds()
	if th.BlockThreshold != 0.9 {
		t.Errorf("default block threshold = %f, want 0.9", th.BlockThreshold)
	}
	if th.ReviewThreshold != 0.5 {
		t.Errorf("default review threshold = %f, want 0.5", th.ReviewThreshold)
	}
}

func TestConfigManager_UpdateThresholds(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	mgr := NewConfigManager(loader, DefaultThresholdConfig())

	// 有效更新
	err := mgr.UpdateThresholds(0.85, 0.3)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	th := mgr.GetThresholds()
	if th.BlockThreshold != 0.85 || th.ReviewThreshold != 0.3 {
		t.Errorf("updated thresholds: block=%f, review=%f", th.BlockThreshold, th.ReviewThreshold)
	}

	// 无效：review >= block
	err = mgr.UpdateThresholds(0.5, 0.6)
	if err == nil {
		t.Error("review >= block should error")
	}

	// 无效：超出 [0,1] 范围
	err = mgr.UpdateThresholds(1.5, 0.3)
	if err == nil {
		t.Error("block > 1 should error")
	}

	err = mgr.UpdateThresholds(0.9, -0.1)
	if err == nil {
		t.Error("review < 0 should error")
	}
}

func TestConfigManager_Decide(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	mgr := NewConfigManager(loader, DefaultThresholdConfig())

	tests := []struct {
		name string
		prob float32
		want types.Result
	}{
		{"PASS", 0.1, types.ResultPass},
		{"PASS-边界", 0.49, types.ResultPass},
		{"REVIEW", 0.5, types.ResultReview},
		{"REVIEW-中", 0.8, types.ResultReview},
		{"BLOCK", 0.9, types.ResultBlock},
		{"BLOCK-高", 0.99, types.ResultBlock},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.Decide(tt.prob)
			if got != tt.want {
				t.Errorf("Decide(%f) = %v, want %v", tt.prob, got, tt.want)
			}
		})
	}

	// 动态调整阈值后重新决策
	_ = mgr.UpdateThresholds(0.7, 0.3)
	got := mgr.Decide(0.5)
	if got != types.ResultReview {
		t.Errorf("after threshold update, Decide(0.5) = %v, want REVIEW", got)
	}
}

func TestConfigManager_ReloadLoader(t *testing.T) {
	oldLoader := NewMockLoader("v1.0-old")
	mgr := NewConfigManager(oldLoader, DefaultThresholdConfig())

	if mgr.ReloadCount() != 0 {
		t.Errorf("initial reload count = %d, want 0", mgr.ReloadCount())
	}

	// 重载新 loader
	newLoader := NewMockLoader("v1.0-new")
	err := mgr.ReloadLoader(newLoader)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if mgr.ReloadCount() != 1 {
		t.Errorf("reload count = %d, want 1", mgr.ReloadCount())
	}

	// 验证当前 loader 是新的
	current := mgr.GetLoader()
	if current.Version() != "v1.0-new" {
		t.Errorf("loader version = %s, want v1.0-new", current.Version())
	}
}

func TestConfigManager_ConcurrentAccess(t *testing.T) {
	loader := NewMockLoader("v1.0-concurrent")
	mgr := NewConfigManager(loader, DefaultThresholdConfig())

	var wg sync.WaitGroup

	// 并发读
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.GetThresholds()
			_ = mgr.Decide(0.7)
			_ = mgr.GetLoader()
		}()
	}

	// 并发写
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			block := float32(0.8) + float32(i%10)*0.01
			_ = mgr.UpdateThresholds(block, 0.3)
		}(i)
	}

	wg.Wait()

	// 最终状态应有效
	th := mgr.GetThresholds()
	if th.ReviewThreshold >= th.BlockThreshold {
		t.Errorf("final thresholds invalid: block=%f, review=%f", th.BlockThreshold, th.ReviewThreshold)
	}
}
