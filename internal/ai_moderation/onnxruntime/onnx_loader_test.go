// Package onnxruntime - onnx_loader_test.go 测试 ONNX loader 决策/softmax/factory 等纯函数。
//
// 注：完整端到端 ONNX 推理测试需要 libonnxruntime + .onnx 模型文件，不在 CI 中运行。
// 这里只测试不依赖 cgo 库的纯逻辑。

//go:build onnx_enabled
// +build onnx_enabled

package onnxruntime

import (
	"testing"

	"go_projects/praProject1/internal/ai_moderation/types"
)

func TestDecideResult(t *testing.T) {
	tests := []struct {
		prob     float32
		expected types.Result
	}{
		{0.95, types.ResultBlock},
		{0.9, types.ResultBlock},
		{0.89, types.ResultReview},
		{0.5, types.ResultReview},
		{0.49, types.ResultPass},
		{0.1, types.ResultPass},
		{0.0, types.ResultPass},
	}
	for _, tt := range tests {
		got := decideResult(tt.prob)
		if got != tt.expected {
			t.Errorf("decideResult(%f): expected %v, got %v", tt.prob, tt.expected, got)
		}
	}
}

func TestSoftmaxAndMax(t *testing.T) {
	// 等概率 → ~0.5
	prob, err := softmaxAndMax([]float32{0.0, 0.0})
	if err != nil {
		t.Fatalf("softmax failed: %v", err)
	}
	if prob < 0.49 || prob > 0.51 {
		t.Errorf("equal logits should give ~0.5, got %f", prob)
	}

	// 第二个 logits 远大 → 接近 1.0
	prob, err = softmaxAndMax([]float32{0.0, 10.0})
	if err != nil {
		t.Fatalf("softmax failed: %v", err)
	}
	if prob < 0.99 {
		t.Errorf("large positive should give ~1.0, got %f", prob)
	}

	// 第二个 logits 远小 → 接近 0
	prob, err = softmaxAndMax([]float32{0.0, -10.0})
	if err != nil {
		t.Fatalf("softmax failed: %v", err)
	}
	if prob > 0.01 {
		t.Errorf("large negative should give ~0, got %f", prob)
	}
}

func TestSoftmaxAndMax_TooFewLogits(t *testing.T) {
	_, err := softmaxAndMax([]float32{0.0})
	if err == nil {
		t.Error("expected error for too few logits")
	}
}

func TestExpFast(t *testing.T) {
	// exp(0) = 1
	if v := expFast(0.0); v < 0.99 || v > 1.01 {
		t.Errorf("exp(0) should be ~1, got %f", v)
	}
	// exp(1) ≈ 2.718
	if v := expFast(1.0); v < 2.5 || v > 3.0 {
		t.Errorf("exp(1) should be ~2.718, got %f", v)
	}
	// exp(-10) 应该非常小（~4.5e-5）
	if v := expFast(-10.0); v > 1e-3 || v < 0 {
		t.Errorf("exp(-10) should be ~0, got %g", v)
	}
}

func TestTryCreateOnnxLoader_DisabledMode(t *testing.T) {
	cfg := types.ModelConfig{Enabled: false, ModelPath: "/tmp/none.onnx"}
	_, err := TryCreateOnnxLoader(cfg)
	if err == nil {
		t.Error("disabled mode should error")
	}
}

func TestTryCreateOnnxLoader_MissingPath(t *testing.T) {
	cfg := types.ModelConfig{Enabled: true, ModelPath: ""}
	_, err := TryCreateOnnxLoader(cfg)
	if err == nil {
		t.Error("missing path should error")
	}
}

func TestTryCreateOnnxLoader_FileNotFound(t *testing.T) {
	cfg := types.ModelConfig{
		Enabled:      true,
		ModelPath:    "/tmp/nonexistent_model.onnx",
		ModelVersion: "v1.0-test",
		TimeoutMs:    800,
	}
	_, err := TryCreateOnnxLoader(cfg)
	if err == nil {
		t.Error("nonexistent file should error")
	}
}

func TestOnnxLoader_Version(t *testing.T) {
	// 不实际加载，只测试 Version 方法（创建后立即销毁）
	// 由于无法轻易 mock onnx session，这里跳过完整测试
	t.Skip("requires ONNX runtime + model file")
}