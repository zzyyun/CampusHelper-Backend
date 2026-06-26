package aiclient

import (
	"context"
	"errors"
	"testing"
	"time"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"
)

// ─── Circuit Breaker 测试 ───────────────────────────────────────────────────

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitConfig())
	if cb.State() != "closed" {
		t.Errorf("expected initial state 'closed', got %q", cb.State())
	}
	if cb.IsOpen() {
		t.Error("circuit should not be open initially")
	}
}

func TestCircuitBreaker_OpenAfterConsecutiveFailures(t *testing.T) {
	cfg := DefaultCircuitConfig()
	cfg.Timeout = 100 * time.Millisecond
	cb := NewCircuitBreaker(cfg)

	// 模拟连续 5 次失败
	for i := 0; i < 5; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("simulated failure")
		})
	}

	// 熔断应已开启
	if !cb.IsOpen() {
		t.Error("circuit should be open after 5 consecutive failures")
	}
}

func TestCircuitBreaker_OpenFastFallback(t *testing.T) {
	cfg := DefaultCircuitConfig()
	cfg.Timeout = 1 * time.Second
	cb := NewCircuitBreaker(cfg)

	// 触发熔断
	for i := 0; i < 5; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}

	// 熔断期间调用应立即返回（<10ms）fallback
	start := time.Now()
	_, err := cb.Execute(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond) // 模拟慢调用
		return "should not execute", nil
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error during open circuit")
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("expected fast fallback (<50ms), got %v", elapsed)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cfg := DefaultCircuitConfig()
	cfg.Timeout = 50 * time.Millisecond
	cb := NewCircuitBreaker(cfg)

	// 触发熔断
	for i := 0; i < 5; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}
	if !cb.IsOpen() {
		t.Fatal("setup: circuit should be open")
	}

	// 等待熔断超时
	time.Sleep(100 * time.Millisecond)

	// 半开状态试探成功 → 关闭
	_, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	if err != nil {
		t.Errorf("half-open probe should succeed, got %v", err)
	}
	if cb.IsOpen() {
		t.Error("circuit should be closed after successful probe")
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitConfig())

	// 3 次成功 + 2 次失败
	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return "ok", nil
		})
	}
	for i := 0; i < 2; i++ {
		_, _ = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
	}

	m := cb.GetMetrics()
	if m.CallsTotal != 5 {
		t.Errorf("expected 5 total calls, got %d", m.CallsTotal)
	}
	if m.SuccessesTotal != 3 {
		t.Errorf("expected 3 successes, got %d", m.SuccessesTotal)
	}
	if m.FailuresTotal != 2 {
		t.Errorf("expected 2 failures, got %d", m.FailuresTotal)
	}
}

// ─── Fallback Response 测试 ────────────────────────────────────────────────

func TestNewFallbackResponse(t *testing.T) {
	resp := NewFallbackResponse("v1.0-mock")
	if resp.Result != ai_moderation_pb.ModerateTextResponse_PASS {
		t.Errorf("fallback should be PASS, got %v", resp.Result)
	}
	if resp.Status != ai_moderation_pb.ModerateTextResponse_DEGRADED {
		t.Errorf("fallback should be DEGRADED, got %v", resp.Status)
	}
	if !resp.FallbackUsed {
		t.Error("fallback should set FallbackUsed=true")
	}
	if resp.ModelVersion != "v1.0-mock" {
		t.Errorf("version mismatch: %s", resp.ModelVersion)
	}
}

// ─── Client 集成测试（不需要真实 ai-moderation 服务）───────────────────────

func TestModerationClient_NilConnection(t *testing.T) {
	// 测试 NewClient 参数校验
	_, err := NewClient(Config{Addr: ""})
	if err == nil {
		t.Error("expected error for empty addr")
	}
}

func TestModerationClient_DefaultTimeout(t *testing.T) {
	cfg := Config{Addr: "127.0.0.1:50061"}
	// 不调用 NewClient 实际连接，仅验证 cfg 构造
	if cfg.Timeout != 0 {
		t.Errorf("timeout should default to 0 (will be set in NewClient)")
	}
	// 默认 800ms 是在 NewClient 内设置的
}

// ─── Context trace_id 测试 ──────────────────────────────────────────────────

func TestContextTraceID(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	traceID, _ := ctx.Value("trace_id").(string)
	if traceID != "trace-abc-123" {
		t.Errorf("trace_id extraction failed: %s", traceID)
	}
}

func TestContextMissingTraceID(t *testing.T) {
	ctx := context.Background()
	traceID, _ := ctx.Value("trace_id").(string)
	if traceID != "" {
		t.Errorf("missing trace_id should be empty, got %s", traceID)
	}
}