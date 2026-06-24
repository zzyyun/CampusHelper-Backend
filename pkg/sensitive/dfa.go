// Package sensitive 提供 DFA（确定有限自动机）多模式敏感词匹配。
//
// 核心特性：
//   - 基于 Trie 前缀树，一次扫描 O(n) 匹配所有模式
//   - 正确支持中英文混合（rune 级别匹配）
//   - 读写锁保护，并发安全
//   - 返回字节偏移量，与 pb.SensitiveWordHit 兼容
package sensitive

import (
	"strings"
	"sync"
)

// TrieNode DFA 前缀树节点。
type TrieNode struct {
	Children map[rune]*TrieNode // 子节点（rune → 节点）
	IsEnd    bool               // 是否为某个敏感词结尾
	Word     string             // 完整敏感词（仅 IsEnd=true 时有效）
}

// DFAMatcher DFA 敏感词匹配器，并发安全。
type DFAMatcher struct {
	root *TrieNode
	mu   sync.RWMutex
}

// NewDFAMatcher 创建新的 DFA 匹配器。
func NewDFAMatcher() *DFAMatcher {
	return &DFAMatcher{
		root: &TrieNode{Children: make(map[rune]*TrieNode)},
	}
}

// Build 用词表构建（或重建）Trie 树。
// 并发安全：写锁阻塞所有 Match 调用直至构建完毕。
func (d *DFAMatcher) Build(words []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.root = &TrieNode{Children: make(map[rune]*TrieNode)}
	for _, word := range words {
		w := strings.TrimSpace(word)
		if w == "" {
			continue
		}
		d.insert(w)
	}
}

// insert 向 Trie 树中插入一个敏感词（非并发安全，调用方需持有锁）。
func (d *DFAMatcher) insert(word string) {
	node := d.root
	for _, ch := range word {
		child, ok := node.Children[ch]
		if !ok {
			child = &TrieNode{Children: make(map[rune]*TrieNode)}
			node.Children[ch] = child
		}
		node = child
	}
	node.IsEnd = true
	node.Word = word
}

// MatchResult 一次命中记录（字节偏移，兼容 pb.SensitiveWordHit）。
type MatchResult struct {
	Word   string // 命中的敏感词
	Start  int    // 命中起始位置（字节偏移）
	End    int    // 命中结束位置（字节偏移，不含）
	Length int    // 命中词长度（字节数，与 Start/End 一致）
}

// Match 在文本中匹配所有敏感词，返回按起始位置排序的命中列表。
//
// 算法：
//   对文本中每个 rune 位置作为起点，沿 Trie 向下遍历；
//   每命中一个 IsEnd 节点即记录一条 MatchResult。
//
// 时间复杂度 O(n * maxWordLen)，实际接近 O(n)（maxWordLen 是常量）。
// 并发安全（读锁）。
func (d *DFAMatcher) Match(text string) []MatchResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(text) == 0 || len(d.root.Children) == 0 {
		return nil
	}

	runes := []rune(text)
	// 预计算每个 rune 位置的字节偏移，用于将 rune 索引映射回字节偏移
	byteOffsets := make([]int, len(runes)+1)
	offset := 0
	for i, r := range runes {
		byteOffsets[i] = offset
		offset += len(string(r)) // 当前 rune 的 UTF-8 编码字节数
	}
	byteOffsets[len(runes)] = offset

	var results []MatchResult

	for i := 0; i < len(runes); i++ {
		node := d.root
		for j := i; j < len(runes); j++ {
			child, ok := node.Children[runes[j]]
			if !ok {
				break // 当前路径无匹配，跳出内层循环
			}
			if child.IsEnd {
				results = append(results, MatchResult{
					Word:   child.Word,
					Start:  byteOffsets[i],
					End:    byteOffsets[j+1],                        // 结束字节偏移（不含）
					Length: byteOffsets[j+1] - byteOffsets[i],        // 字节长度（与 Start/End 一致）
				})
			}
			node = child
		}
	}
	return results
}

// MatchCount 仅返回命中数量（不构造结果列表，适合快速检查）。
func (d *DFAMatcher) MatchCount(text string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(text) == 0 || len(d.root.Children) == 0 {
		return 0
	}

	runes := []rune(text)
	count := 0
	for i := 0; i < len(runes); i++ {
		node := d.root
		for j := i; j < len(runes); j++ {
			child, ok := node.Children[runes[j]]
			if !ok {
				break
			}
			if child.IsEnd {
				count++
			}
			node = child
		}
	}
	return count
}