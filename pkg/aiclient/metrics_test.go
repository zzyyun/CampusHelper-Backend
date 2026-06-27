package aiclient

import "testing"

func TestUpdateCircuitState(t *testing.T) {
	// 测试不会 panic
	UpdateCircuitState("closed")
	UpdateCircuitState("open")
	UpdateCircuitState("half-open")
	UpdateCircuitState("unknown")
}

func TestCircuitState_Metric(t *testing.T) {
	// 验证 metric 注册成功
	c := CircuitState
	if c == nil {
		t.Fatal("CircuitState metric not registered")
	}
	// 简单的 Inc/Set/Dec 操作
	c.Set(1)
	c.Set(0)
	c.Inc()
	c.Dec()
}