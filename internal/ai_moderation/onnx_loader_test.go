package ai_moderation

import (
	"testing"
)

func TestDecideResult(t *testing.T) {
	tests := []struct {
		prob     float32
		expected Result
	}{
		{0.95, ResultBlock},
		{0.9, ResultBlock},
		{0.89, ResultReview},
		{0.5, ResultReview},
		{0.49, ResultPass},
		{0.1, ResultPass},
		{0.0, ResultPass},
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

func TestBertTokenizer_Encode(t *testing.T) {
	tok, err := NewBertTokenizer()
	if err != nil {
		t.Fatalf("tokenizer init: %v", err)
	}

	inputIDs, mask, err := tok.Encode("hello world 你好", 64)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if len(inputIDs) != 64 {
		t.Errorf("expected length 64, got %d", len(inputIDs))
	}
	if len(mask) != 64 {
		t.Errorf("mask length mismatch: %d", len(mask))
	}
	if inputIDs[0] != 101 {
		t.Errorf("first token should be [CLS]=101, got %d", inputIDs[0])
	}
	if inputIDs[len(inputIDs)-1] != 0 {
		// padding token = 0
		t.Errorf("last non-special token should be padding=0, got %d", inputIDs[len(inputIDs)-1])
	}
	// [CLS] 和首字符应是 attention mask = 1
	if mask[0] != 1 {
		t.Errorf("first mask should be 1, got %d", mask[0])
	}
}

func TestBertTokenizer_Truncate(t *testing.T) {
	tok, _ := NewBertTokenizer()
	longText := ""
	for i := 0; i < 1000; i++ {
		longText += "你"
	}
	ids, _, err := tok.Encode(longText, 64)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(ids) != 64 {
		t.Errorf("should truncate to 64, got %d", len(ids))
	}
}

func TestTryCreateOnnxLoader_DisabledMode(t *testing.T) {
	cfg := ModelConfig{Enabled: false, ModelPath: "/tmp/none.onnx"}
	_, err := TryCreateOnnxLoader(cfg)
	if err == nil {
		t.Error("disabled mode should error")
	}
}

func TestTryCreateOnnxLoader_MissingPath(t *testing.T) {
	cfg := ModelConfig{Enabled: true, ModelPath: ""}
	_, err := TryCreateOnnxLoader(cfg)
	if err == nil {
		t.Error("missing path should error")
	}
}

func TestTryCreateOnnxLoader_FileNotFound(t *testing.T) {
	cfg := ModelConfig{
		Enabled:    true,
		ModelPath:  "/tmp/nonexistent_model.onnx",
		ModelVersion: "v1.0-test",
		TimeoutMs: 800,
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