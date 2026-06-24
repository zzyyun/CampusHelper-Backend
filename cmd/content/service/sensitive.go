package service

import (
	pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/pkg/sensitive"
)

// ─── 敏感词 DFA 扫描器 ─────────────────────────────────────────────────────
//
// Issue #4 升级：将 Phase 1 的简单 strings.Index 遍历替换为真正的 DFA
// (Deterministic Finite Automaton) 多模式匹配算法。
//
// 设计：
//   - 基于 pkg/sensitive.DFAMatcher（Trie 前缀树 + O(n) 扫描）
//   - 内置灰词表用于快速启动；Phase 2 接入 Redis 词库（WordManager）热更新
//   - 命中后返回具体位置（字节偏移 start/end + 字符长度 length）
//
// 词表维护原则：
//   - 严禁硬编码政治/宗教等高度敏感词（避免仓库泄露风险）
//   - 灰词（广告/联系方式）可保留：手机号 / 微信号 / QQ

// defaultWords 内置灰词表（Phase 1 退路）。
var defaultWords = []string{
	"微信号", "加我微信", "联系电话",
	"http://", "https://",
}

// dfaMatcher 全局 DFA 匹配器，服务启动时构建完成。
var dfaMatcher = newDFAMatcher()

// newDFAMatcher 用内置词表初始化 DFA 匹配器。
func newDFAMatcher() *sensitive.DFAMatcher {
	m := sensitive.NewDFAMatcher()
	m.Build(defaultWords)
	return m
}

// ScanSensitive 扫描文本中的敏感词，返回所有命中位置（与 pb.SensitiveWordHit 兼容）。
// 返回 nil 表示无命中。
func ScanSensitive(text string) []*pb.SensitiveWordHit {
	if text == "" {
		return nil
	}
	results := dfaMatcher.Match(text)
	if len(results) == 0 {
		return nil
	}
	hits := make([]*pb.SensitiveWordHit, len(results))
	for i, r := range results {
		hits[i] = &pb.SensitiveWordHit{
			Word:   r.Word,
			Start:  int32(r.Start),
			End:    int32(r.End),
			Length: int32(r.Length),
		}
	}
	return hits
}

// ReloadWords 从外部词表重建 DFA 匹配器（供 Redis 热更新回调使用）。
func ReloadWords(words []string) {
	dfaMatcher.Build(words)
}