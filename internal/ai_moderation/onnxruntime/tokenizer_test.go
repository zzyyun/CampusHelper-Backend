// Package onnxruntime - tokenizer_test.go 测试 BERT tokenizer 纯函数（不依赖 cgo）。

//go:build onnx_enabled
// +build onnx_enabled

package onnxruntime

import (
	"testing"
)

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