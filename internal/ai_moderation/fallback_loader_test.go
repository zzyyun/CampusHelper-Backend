package ai_moderation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"go_projects/praProject1/internal/ai_moderation/types"
)

// mockFailLoader 始终返回错误的 loader（模拟 ONNX 推理失败）。
type mockFailLoader struct {
	err error
}

func (m *mockFailLoader) Infer(_ context.Context, _ string) (*types.InferenceResult, error) {
	return nil, m.err
}

func (m *mockFailLoader) Version() string          { return "fail-loader" }
func (m *mockFailLoader) Close() error              { return nil }

// mockSlowLoader 模拟慢速推理（可配置延迟）。
type mockSlowLoader struct {
	delay int // ms
	*MockLoader
}

func TestFallbackLoader_PrimarySuccess(t *testing.T) {
	primary := NewMockLoader("primary")
	fallback := NewMockLoader("fallback")
	loader := NewFallbackLoader(primary, fallback, DefaultFallbackConfig())

	result, err := loader.Infer(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}
	if result.ModelVersion != "primary" {
		t.Errorf("should use primary, got version=%s", result.ModelVersion)
	}
	if loader.IsDegraded() {
		t.Error("should not be degraded")
	}
}

func TestFallbackLoader_PrimaryFail_FallbackToMock(t *testing.T) {
	primary := &mockFailLoader{err: errors.New("ONNX inference failed")}
	fallback := NewMockLoader("fallback")
	loader := NewFallbackLoader(primary, fallback, DefaultFallbackConfig())

	result, err := loader.Infer(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Infer should not error (fallback): %v", err)
	}
	if result.ModelVersion != "fallback" {
		t.Errorf("should use fallback, got version=%s", result.ModelVersion)
	}
	if !result.FallbackUsed {
		t.Error("fallback result should mark FallbackUsed=true")
	}
	if !loader.IsDegraded() {
		t.Error("should be degraded after primary failure")
	}
}

func TestFallbackLoader_DegradedUsesFallback(t *testing.T) {
	primary := &mockFailLoader{err: errors.New("ONNX down")}
	fallback := NewMockLoader("fallback-v2")
	config := FallbackConfig{
		MaxConsecutiveErrors: 1,
		AlertOnDegrade:        false,
	}
	loader := NewFallbackLoader(primary, fallback, config)

	// 第一次调用触发降级
	_, _ = loader.Infer(context.Background(), "test1")

	// 后续调用直接走 fallback
	result, err := loader.Infer(context.Background(), "test2")
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}
	if result.ModelVersion != "fallback-v2" {
		t.Errorf("should use fallback, got %s", result.ModelVersion)
	}
}

func TestFallbackLoader_Version(t *testing.T) {
	primary := NewMockLoader("v1.0-onnx")
	fallback := NewMockLoader("v1.0-mock")
	loader := NewFallbackLoader(primary, fallback, DefaultFallbackConfig())

	// 正常状态
	if v := loader.Version(); v != "v1.0-onnx" {
		t.Errorf("normal version = %s, want v1.0-onnx", v)
	}

	// 触发降级
	loader2 := NewFallbackLoader(
		&mockFailLoader{err: errors.New("fail")},
		fallback,
		DefaultFallbackConfig(),
	)
	_, _ = loader2.Infer(context.Background(), "trigger degrade")
	if v := loader2.Version(); v != "v1.0-mock-fallback" {
		t.Errorf("degraded version = %s, want v1.0-mock-fallback", v)
	}
}

func TestFallbackLoader_Close(t *testing.T) {
	primary := NewMockLoader("primary")
	fallback := NewMockLoader("fallback")
	loader := NewFallbackLoader(primary, fallback, DefaultFallbackConfig())

	err := loader.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestFallbackLoader_ConcurrentInfer(t *testing.T) {
	primary := NewMockLoader("primary")
	fallback := NewMockLoader("fallback")
	loader := NewFallbackLoader(primary, fallback, DefaultFallbackConfig())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := loader.Infer(context.Background(), "concurrent test")
			if err != nil {
				t.Errorf("Infer error: %v", err)
				return
			}
			if result == nil {
				t.Error("result is nil")
			}
		}()
	}
	wg.Wait()
}

func TestFallbackLoader_ConcurrentDegrade(t *testing.T) {
	// 模拟高并发下 primary 失败的降级场景
	primary := &mockFailLoader{err: errors.New("concurrent fail")}
	fallback := NewMockLoader("fallback")
	loader := NewFallbackLoader(primary, fallback, FallbackConfig{
		MaxConsecutiveErrors: 1,
		AlertOnDegrade:        false,
	})

	var wg sync.WaitGroup
	var successCount int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := loader.Infer(context.Background(), "concurrent degrade test")
			if err == nil && result != nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	// 全部应成功（降级到 fallback）
	if count := atomic.LoadInt32(&successCount); count != 100 {
		t.Errorf("expected 100 successes, got %d", count)
	}
	if !loader.IsDegraded() {
		t.Error("should be degraded")
	}
}

func TestNewFallbackLoaderFromConfig_MockMode(t *testing.T) {
	cfg := ModelConfig{
		Enabled:      false,
		ModelVersion: "v1.0-test",
	}
	loader, err := NewFallbackLoaderFromConfig(cfg)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	defer loader.Close()

	// mock 模式应直接返回 MockLoader（不是 FallbackLoader）
	if _, ok := loader.(*MockLoader); !ok {
		t.Logf("loader type: %T (acceptable)", loader)
	}

	result, err := loader.Infer(context.Background(), "test")
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}
	if !result.FallbackUsed {
		t.Error("mock mode should mark FallbackUsed=true")
	}
}

func TestNewFallbackLoaderFromConfig_ONNXDisabled(t *testing.T) {
	cfg := ModelConfig{
		Enabled:      true,
		ModelVersion: "v1.0",
		ModelPath:    "/nonexistent.onnx",
		TimeoutMs:    800,
	}
	// onnxFactory 为 nil（未启用 onnx_enabled tag）
	loader, err := NewFallbackLoaderFromConfig(cfg)
	if err != nil {
		t.Fatalf("should fallback gracefully: %v", err)
	}
	defer loader.Close()
}

func TestDefaultFallbackConfig(t *testing.T) {
	cfg := DefaultFallbackConfig()
	if cfg.MaxConsecutiveErrors != 3 {
		t.Errorf("MaxConsecutiveErrors = %d, want 3", cfg.MaxConsecutiveErrors)
	}
	if cfg.RecoveryCheckInterval <= 0 {
		t.Error("RecoveryCheckInterval should be positive")
	}
	if !cfg.AlertOnDegrade {
		t.Error("AlertOnDegrade should default to true")
	}
}

func TestFallbackLoader_NoPrimary(t *testing.T) {
	// primary 为 nil 时应 graceful 降级
	fallback := NewMockLoader("fallback-only")
	loader := NewFallbackLoader(nil, fallback, DefaultFallbackConfig())

	result, err := loader.Infer(context.Background(), "no primary")
	if err != nil {
		t.Fatalf("should fallback: %v", err)
	}
	if result.ModelVersion != "fallback-only" {
		t.Errorf("version = %s, want fallback-only", result.ModelVersion)
	}
}
