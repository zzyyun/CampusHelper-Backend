// Package onnxruntime - onnx_loader_test.go 测试 ONNX loader 决策/softmax 等纯函数。
//
// 注：完整端到端 ONNX 推理测试需要 libonnxruntime + .onnx 模型文件，不在 CI 中运行。

//go:build onnx_enabled
// +build onnx_enabled

package onnxruntime

import (
	"testing"

	"go_projects/praProject1/internal/ai_moderation/types"
)

func TestDecideResult(t *testing.T) {
	tests := []struct {
		name string
		prob float32
		want types.Result
	}{
		{"BLOCK - 高概率", 0.95, types.ResultBlock},
		{"BLOCK - 边界", 0.9, types.ResultBlock},
		{"REVIEW - 中高", 0.89, types.ResultReview},
		{"REVIEW - 中概率", 0.5, types.ResultReview},
		{"PASS - 边界", 0.49, types.ResultPass},
		{"PASS - 低概率", 0.1, types.ResultPass},
		{"PASS - 零", 0.0, types.ResultPass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideResult(tt.prob)
			if got != tt.want {
				t.Errorf("decideResult(%f): expected %v, got %v", tt.prob, tt.want, got)
			}
		})
	}
}

func TestSoftmax6(t *testing.T) {
	tests := []struct {
		name   string
		logits []float32
	}{
		{"均匀分布", []float32{1, 1, 1, 1, 1, 1}},
		{"第一个最高", []float32{10, 0, 0, 0, 0, 0}},
		{"最后一个最高", []float32{0, 0, 0, 0, 0, 10}},
		{"混合", []float32{1, 5, 2, 3, 8, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probs := softmax6(tt.logits)
			if len(probs) != 6 {
				t.Fatalf("softmax6 returned %d values, want 6", len(probs))
			}

			// 概率总和应为 1.0
			sum := float32(0)
			for _, p := range probs {
				sum += p
			}
			if sum < 0.99 || sum > 1.01 {
				t.Errorf("softmax6 sum = %f, want ~1.0", sum)
			}

			// 所有概率应在 [0, 1] 范围内
			for i, p := range probs {
				if p < 0 || p > 1 {
					t.Errorf("probs[%d] = %f, want [0, 1]", i, p)
				}
			}
		})
	}
}

func TestSoftmax6_LogitsTooShort(t *testing.T) {
	// 不足 6 个元素时应自动补零
	probs := softmax6([]float32{1.0, 2.0})
	if len(probs) != 6 {
		t.Fatalf("softmax6 should pad to 6, got %d", len(probs))
	}
	sum := float32(0)
	for _, p := range probs {
		sum += p
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("softmax6 sum = %f, want ~1.0", sum)
	}
}

func TestMaxWithIndex(t *testing.T) {
	tests := []struct {
		name    string
		vals    []float32
		wantMax float32
		wantIdx int
	}{
		{"第一个最大", []float32{5, 3, 1}, 5, 0},
		{"中间最大", []float32{1, 5, 3}, 5, 1},
		{"最后最大", []float32{1, 3, 5}, 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxVal, maxIdx := maxWithIndex(tt.vals)
			if maxVal != tt.wantMax || maxIdx != tt.wantIdx {
				t.Errorf("maxWithIndex(%v) = (%f, %d), want (%f, %d)",
					tt.vals, maxVal, maxIdx, tt.wantMax, tt.wantIdx)
			}
		})
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
