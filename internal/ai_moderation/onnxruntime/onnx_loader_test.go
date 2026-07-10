// Package onnxruntime - onnx_loader_test.go 测试 ONNX loader ���策/softmax 等纯函数。
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
		{"REVIEW - 中��率", 0.5, types.ResultReview},
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

func TestSoftmax(t *testing.T) {
	tests := []struct {
		name   string
		logits []float32
	}{
		{"2分类均匀", []float32{1, 1}},
		{"3分类第一个最高", []float32{10, 0, 0}},
		{"3分类最后一个最��", []float32{0, 0, 10}},
		{"6分类混��", []float32{1, 5, 2, 3, 8, 4}},
		{"2分类极值", []float32{1000, -1000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probs := softmax(tt.logits)
			if len(probs) != len(tt.logits) {
				t.Fatalf("softmax returned %d values, want %d", len(probs), len(tt.logits))
			}

			// 概���总和应为 1.0
			sum := float32(0)
			for _, p := range probs {
				sum += p
			}
			if sum < 0.99 || sum > 1.01 {
				t.Errorf("softmax sum = %f, want ~1.0, logits=%v", sum, tt.logits)
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

func TestSoftmax_EmptyInput(t *testing.T) {
	// 空输入应返回 nil
	probs := softmax([]float32{})
	if probs != nil {
		t.Errorf("empty input should return nil, got %v", probs)
	}
}

func TestSoftmax_BinaryClassification(t *testing.T) {
	// 二分类（中文内容审核模型���：logits → 安全/违规
	// 违规 logit 高 = 高违规概率
	probs := softmax([]float32{-2.0, 5.0})
	if len(probs) != 2 {
		t.Fatalf("binary softmax returned %d values, want 2", len(probs))
	}
	// 第 2 类（违规）概率应接近 1.0
	if probs[1] < 0.99 {
		t.Errorf("violation prob = %f, want ~1.0 for logit=5.0", probs[1])
	}
	// 概率和 = 1.0
	if probs[0]+probs[1] < 0.99 || probs[0]+probs[1] > 1.01 {
		t.Errorf("binary softmax sum = %f, want ~1.0", probs[0]+probs[1])
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

// ─── TDD 边界测试 ────────────────────────────────────────────────────────────

// softmax 极端值：极大 logits 不应溢出为 Inf/NaN
func TestSoftmax_ExtremeValues(t *testing.T) {
	probs := softmax([]float32{1000, -1000, 0, 0, 0, 0})
	sum := float32(0)
	for i, p := range probs {
		if p < 0 || p > 1 {
			t.Errorf("probs[%d] = %f out of [0,1]", i, p)
		}
		sum += p
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("extreme softmax sum = %f, want ~1.0", sum)
	}
	// 第一个应该接近 1.0（几乎全部概率���中）
	if probs[0] < 0.99 {
		t.Errorf("probs[0] = %f, want ~1.0 for extreme logit", probs[0])
	}
}

// softmax 2 分类极值
func TestSoftmax_2ClassExtreme(t *testing.T) {
	probs := softmax([]float32{-1000, 1000})
	if len(probs) != 2 {
		t.Fatalf("expected 2 values, got %d", len(probs))
	}
	// 第 2 类应接近 1.0
	if probs[1] < 0.99 {
		t.Errorf("probs[1] = %f, want ~1.0", probs[1])
	}
	sum := probs[0] + probs[1]
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("softmax sum = %f, want ~1.0", sum)
	}
}

// maxWithIndex 边界：空切片
func TestMaxWithIndex_Empty(t *testing.T) {
	maxVal, maxIdx := maxWithIndex([]float32{})
	if maxVal != 0 || maxIdx != 0 {
		t.Errorf("empty: got (%f, %d), want (0, 0)", maxVal, maxIdx)
	}
}

// maxWithIndex 所有值相同：应返回第一个
func TestMaxWithIndex_AllEqual(t *testing.T) {
	maxVal, maxIdx := maxWithIndex([]float32{5, 5, 5})
	if maxVal != 5 || maxIdx != 0 {
		t.Errorf("all equal: got (%f, %d), want (5, 0)", maxVal, maxIdx)
	}
}

// decideResult 极限边界：1.0 应 BLOCK，-0.0 应 PASS
func TestDecideResult_Extremes(t *testing.T) {
	if got := decideResult(1.0); got != types.ResultBlock {
		t.Errorf("decideResult(1.0) = %v, want BLOCK", got)
	}
	if got := decideResult(-0.0); got != types.ResultPass {
		t.Errorf("decideResult(-0.0) = %v, want PASS", got)
	}
}

// softmax → decideResult 全链路：验证中文二分类时正���
func TestSoftmaxToDecideResult_ChineseBinary(t *testing.T) {
	// 模拟中文内容审核模型：第 2 类（违规）概率高
	// logits: [安全=0.5, 违规=5.0]
	logits := []float32{0.5, 5.0}
	probs := softmax(logits)
	maxProb, maxIdx := maxWithIndex(probs)

	if maxIdx != 1 {
		t.Errorf("maxIdx = %d, want 1 (violation)", maxIdx)
	}
	if maxProb < 0.98 {
		t.Errorf("maxProb = %f, want >0.98", maxProb)
	}
	result := decideResult(maxProb)
	if result != types.ResultBlock {
		t.Errorf("decideResult(%f) = %v, want BLOCK", maxProb, result)
	}
}

// softmax → decideResult 全链路：验证英文 6 分类时正确映射
func TestSoftmaxToDecideResult_English6Class(t *testing.T) {
	// 模拟 English toxic-bert 模型：insult（index=4）概率最高
	logits := []float32{0, 0, 0, 0, 10, 0}
	probs := softmax(logits)
	maxProb, maxIdx := maxWithIndex(probs)

	if maxIdx != 4 {
		t.Errorf("maxIdx = %d, want 4 (insult)", maxIdx)
	}
	if maxProb < 0.99 {
		t.Errorf("maxProb = %f, want ~1.0", maxProb)
	}
	result := decideResult(maxProb)
	if result != types.ResultBlock {
		t.Errorf("decideResult(%f) = %v, want BLOCK", maxProb, result)
	}
}
