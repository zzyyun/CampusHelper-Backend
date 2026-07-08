//go:build onnx_enabled
// +build onnx_enabled

package onnxruntime

import (
	"os"
	"testing"
)

const testVocabPath = "../../../models/vocab.txt"

func TestNewBertTokenizerFromFile(t *testing.T) {
	if _, err := os.Stat(testVocabPath); os.IsNotExist(err) {
		t.Skipf("vocab.txt not found at %s, skipping", testVocabPath)
	}

	tok, err := NewBertTokenizerFromFile(testVocabPath)
	if err != nil {
		t.Fatalf("NewBertTokenizerFromFile() error: %v", err)
	}

	if _, ok := tok.vocab["[CLS]"]; !ok {
		t.Error("[CLS] token not found in vocab")
	}
	if _, ok := tok.vocab["[SEP]"]; !ok {
		t.Error("[SEP] token not found in vocab")
	}
	if _, ok := tok.vocab["[PAD]"]; !ok {
		t.Error("[PAD] token not found in vocab")
	}
	if _, ok := tok.vocab["[UNK]"]; !ok {
		t.Error("[UNK] token not found in vocab")
	}

	t.Logf("Vocab size: %d, [CLS]=%d, [SEP]=%d, [PAD]=%d, [UNK]=%d",
		len(tok.vocab), tok.clsID, tok.sepID, tok.padID, tok.unkID)
}

func TestBertTokenizer_Encode(t *testing.T) {
	if _, err := os.Stat(testVocabPath); os.IsNotExist(err) {
		t.Skipf("vocab.txt not found at %s, skipping", testVocabPath)
	}

	tok, err := NewBertTokenizerFromFile(testVocabPath)
	if err != nil {
		t.Fatalf("NewBertTokenizerFromFile() error: %v", err)
	}

	tests := []struct {
		name   string
		text   string
		maxLen int
	}{
		{"短英文", "hello world", 128},
		{"空文本", "", 128},
		{"中文", "hello 你好世界", 128},
		{"长文本截断", "this is a very long text that should be truncated at 32 tokens", 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputIDs, attentionMask, tokenTypeIDs, err := tok.Encode(tt.text, tt.maxLen)
			if err != nil {
				t.Fatalf("Encode() error: %v", err)
			}
			if len(inputIDs) != tt.maxLen {
				t.Errorf("inputIDs length = %d, want %d", len(inputIDs), tt.maxLen)
			}
			if len(attentionMask) != tt.maxLen {
				t.Errorf("attentionMask length = %d, want %d", len(attentionMask), tt.maxLen)
			}
			if len(tokenTypeIDs) != tt.maxLen {
				t.Errorf("tokenTypeIDs length = %d, want %d", len(tokenTypeIDs), tt.maxLen)
			}
			// 第一个 token 应该是 [CLS]
			if inputIDs[0] != tok.clsID {
				t.Errorf("first token = %d, want [CLS]=%d", inputIDs[0], tok.clsID)
			}
			// 有效 token 的 attention_mask 应该是 1
			totalOnes := 0
			for _, v := range attentionMask {
				if v == 1 {
					totalOnes++
				}
			}
			if totalOnes == 0 {
				t.Error("attentionMask has no 1s")
			}
			// tokenTypeIDs 应全为 0（单句任务）
			for i, v := range tokenTypeIDs {
				if v != 0 {
					t.Errorf("tokenTypeIDs[%d] = %d, want 0", i, v)
					break
				}
			}
			t.Logf("text=%q: %d tokens, inputIDs[:5]=%v", tt.text, totalOnes, inputIDs[:min(5, len(inputIDs))])
		})
	}
}

func TestWordPieceSplit(t *testing.T) {
	vocab := map[string]int64{
		"[UNK]":  0,
		"un":     1,
		"##able": 2,
		"##ness": 3,
	}

	tests := []struct {
		name  string
		word  string
		wantN int
	}{
		{"整词匹配", "hello", 1},
		{"子词切分", "unable", 2},
		{"完全未知", "xyz", 1}, // [UNK]
		{"空字符串", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wordPieceSplit(tt.word, vocab)
			if len(result) != tt.wantN {
				t.Errorf("wordPieceSplit(%q) = %v (len=%d), want %d", tt.word, result, len(result), tt.wantN)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
