// Package onnxruntime - tokenizer.go 提供 BERT tokenizer 完整实现（仅 onnx_enabled 编译）。
//
// 基于 vocab.txt 的 WordPiece 分词，支持 [CLS]/[SEP]/[PAD]/[UNK] 特殊 token。
// vocab_size=30522（与 toxic-bert 模型一致）。

//go:build onnx_enabled
// +build onnx_enabled

package onnxruntime

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// BertTokenizer 基于 vocab.txt 的 BERT tokenizer。
type BertTokenizer struct {
	vocab  map[string]int64 // token → id
	maxLen int
	unkID  int64
	clsID  int64
	sepID  int64
	padID  int64
}

// NewBertTokenizerFromFile 从 vocab.txt 创建 tokenizer。
func NewBertTokenizerFromFile(vocabPath string) (*BertTokenizer, error) {
	vocab, err := loadVocab(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("load vocab: %w", err)
	}

	t := &BertTokenizer{
		vocab:  vocab,
		maxLen: 512,
		unkID:  vocab["[UNK]"],
		clsID:  vocab["[CLS]"],
		sepID:  vocab["[SEP]"],
		padID:  vocab["[PAD]"],
	}

	if t.clsID == 0 && vocab["[CLS]"] == 0 {
		// [CLS] 可能正好在位置 0，这是合法的
	}
	if _, ok := vocab["[CLS]"]; !ok {
		return nil, fmt.Errorf("vocab missing [CLS] token")
	}

	return t, nil
}

// loadVocab 从 vocab.txt 加载词表（每行一个 token，行号即为 id）。
func loadVocab(path string) (map[string]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := make(map[string]int64, 30522)
	var id int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		token := strings.TrimSpace(scanner.Text())
		if token != "" {
			vocab[token] = id
		}
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vocab, nil
}

// Encode 文本 → input_ids + attention_mask + token_type_ids。
//
// BERT 格式: [CLS] token1 token2 ... [SEP] [PAD] ...
func (t *BertTokenizer) Encode(text string, maxLen int) ([]int64, []int64, []int64, error) {
	if maxLen <= 0 || maxLen > t.maxLen {
		maxLen = t.maxLen
	}

	// 1. 文本清洗
	cleaned := cleanText(text)

	// 2. WordPiece 分词
	tokens := wordPieceTokenize(cleaned, t.vocab)

	// 3. 截断（留 2 个位置给 [CLS] 和 [SEP]）
	if len(tokens) > maxLen-2 {
		tokens = tokens[:maxLen-2]
	}

	// 4. 构造 input_ids: [CLS] + tokens + [SEP]
	inputIDs := make([]int64, 0, maxLen)
	inputIDs = append(inputIDs, t.clsID)
	for _, tok := range tokens {
		id, ok := t.vocab[tok]
		if !ok {
			id = t.unkID // 未知 token → [UNK]
		}
		inputIDs = append(inputIDs, id)
	}
	inputIDs = append(inputIDs, t.sepID)

	// 5. padding 到 maxLen
	tokenLen := len(inputIDs)
	for len(inputIDs) < maxLen {
		inputIDs = append(inputIDs, t.padID)
	}

	// 6. attention_mask: 有效 token=1, padding=0
	attentionMask := make([]int64, maxLen)
	for i := 0; i < tokenLen; i++ {
		attentionMask[i] = 1
	}

	// 7. token_type_ids: 全 0（单句任务）
	tokenTypeIDs := make([]int64, maxLen)

	return inputIDs, attentionMask, tokenTypeIDs, nil
}

// wordPieceTokenize WordPiece 分词。
//
// 英文按空格分词后尝试子词切分，中文按字符切分。
func wordPieceTokenize(text string, vocab map[string]int64) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				tokens = append(tokens, wordPieceSplit(current.String(), vocab)...)
				current.Reset()
			}
		} else if unicode.Is(unicode.Han, r) {
			// 中文字符：先flush已有英文，再逐字符
			if current.Len() > 0 {
				tokens = append(tokens, wordPieceSplit(current.String(), vocab)...)
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, wordPieceSplit(current.String(), vocab)...)
	}

	return tokens
}

// wordPieceSplit 对单个英文 token 做 WordPiece 子词切分。
//
// 贪心最大匹配：从最长后缀开始尝试，找到就切分。
func wordPieceSplit(word string, vocab map[string]int64) []string {
	if len(word) == 0 {
		return nil
	}

	// 整词在词表中，直接返回
	if _, ok := vocab[word]; ok {
		return []string{word}
	}

	var tokens []string
	remaining := word
	for len(remaining) > 0 {
		found := false
		// 从最长子串开始尝试
		for end := len(remaining); end > 0; end-- {
			sub := remaining[:end]
			prefix := ""
			if len(tokens) > 0 {
				prefix = "##"
			}
			candidate := prefix + sub
			if _, ok := vocab[candidate]; ok {
				tokens = append(tokens, candidate)
				remaining = remaining[end:]
				found = true
				break
			}
		}
		if !found {
			// 整个剩余部分无法匹配，标记为 [UNK]
			tokens = append(tokens, "[UNK]")
			break
		}
	}

	return tokens
}

// cleanText 清洗文本（小写 + 去除控制字符）。
//
// 中文特殊处理：保留中文字符原样（��转换为小写以避��损坏）。
func cleanText(text string) string {
	var sb strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		sb.WriteRune(r)
	}
	cleaned := strings.TrimSpace(sb.String())
	// ��非中文文本应用小写转换，中文保持原样
	lower := strings.ToLower(cleaned)
	// 仅当文本中中文字符占比 < 50% 时使用小写（防止中文模型 vocab 匹配失败）
	hanCount := 0
	for _, r := range lower {
		if unicode.Is(unicode.Han, r) {
			hanCount++
		}
	}
	if float64(hanCount)/float64(len([]rune(lower))) >= 0.5 {
		return cleaned // 中文文本：保持原始大小写
	}
	return lower // 英文/AH文本：转为小写
}

// Close 释放 tokenizer 资源（当前为无操作，预留接口）。
func (t *BertTokenizer) Close() error {
	t.vocab = nil
	return nil
}
