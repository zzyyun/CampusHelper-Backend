// Package ai_moderation - tokenizer.go 提供 BERT tokenizer 简化实现。
//
// 注意：本实现为最小可用版本（仅支持 WordPiece + 基础 vocab）。
// 生产环境建议使用 huggingface/tokenizers 或完整 bert tokenizer 库。
//
// 关联：task-046 (#93) ONNX 模型推理
package ai_moderation

import (
	"hash/fnv"
	"strings"
	"unicode"
)

// BertTokenizer 简化版 BERT tokenizer
//
// 与 HuggingFace BERT tokenizer 主要差异：
//   - 本实现使用 hash-based vocab lookup（无完整 vocab 文件）
//   - 适用场景：mock 测试 + 小规模模型验证
//   - 生产应使用完整 vocab.txt + 完整 WordPiece 算法
type BertTokenizer struct {
	maxLen int
	vocabSize int
}

// NewBertTokenizer 创建 tokenizer
func NewBertTokenizer() (*BertTokenizer, error) {
	return &BertTokenizer{
		maxLen:    512,
		vocabSize: 21128, // bert-base-chinese vocab size
	}, nil
}

// Encode 文本 → token IDs + attention mask
//
// 返回：
//   - inputIDs: token id 数组（[CLS] + tokens + [SEP]）
//   - attentionMask: 1 数组（全部为 1，padding 部分为 0）
//   - error: 编码错误
func (t *BertTokenizer) Encode(text string, maxLen int) ([]int64, []int64, error) {
	if maxLen <= 0 || maxLen > 512 {
		maxLen = 512
	}

	// 1. 文本清洗（小写 + 去除控制字符）
	cleaned := cleanText(text)

	// 2. 分词（中文按字符，英文按词）
	tokens := tokenize(cleaned)

	// 3. 限制长度（留 2 个位置给 [CLS] 和 [SEP]）
	if len(tokens) > maxLen-2 {
		tokens = tokens[:maxLen-2]
	}

	// 4. 添加特殊 token
	inputIDs := make([]int64, 0, maxLen)
	inputIDs = append(inputIDs, 101) // [CLS]
	for _, tok := range tokens {
		inputIDs = append(inputIDs, t.tokenToID(tok))
	}
	inputIDs = append(inputIDs, 102) // [SEP]

	// 5. padding
	for len(inputIDs) < maxLen {
		inputIDs = append(inputIDs, 0) // [PAD]
	}

	// 6. attention mask
	attentionMask := make([]int64, maxLen)
	for i := range attentionMask {
		if i < len(inputIDs) && inputIDs[i] != 0 {
			attentionMask[i] = 1
		}
	}

	return inputIDs, attentionMask, nil
}

// tokenToID 将 token 转为 ID（hash-based 简化实现）
//
// 真实实现应查询 vocab.txt 的 WordPiece 词表
// 当前为简化版：使用 FNV hash 映射到 vocab 范围内
func (t *BertTokenizer) tokenToID(token string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return int64(h.Sum32() % uint32(t.vocabSize))
}

// cleanText 清洗文本
func cleanText(text string) string {
	var sb strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		sb.WriteRune(r)
	}
	return strings.ToLower(strings.TrimSpace(sb.String()))
}

// tokenize 简单分词（中文按字符，英文按词）
func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else if unicode.Is(unicode.Han, r) {
			// 中文字符：每个字符独立成 token
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}