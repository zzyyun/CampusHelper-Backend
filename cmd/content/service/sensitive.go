package service

import (
	"strings"

	pb "go_projects/praProject1/PB/pb/content_pb"
)

// ─── 敏感词 DFA 扫描器（Phase 1 占位实现） ───────────────────────────────────
//
// 设计说明：
//   - 完整的 DFA 算法请参考 docs/content-service-prd.md §4.4
//   - Phase 1 仅内置一份小词表用于自测，Phase 2 将词表替换为 RabbitMQ 异步加载
//   - 命中后返回具体位置（start/end/length）便于前端高亮提示
//
// 词表维护原则：
//   - 严禁硬编码政治/宗教等高度敏感词（避免仓库泄露风险）
//   - 灰词（广告/联系方式）可保留：手机号 / 微信号 / QQ

// sensitiveWords 内置灰词表（Phase 1）
var sensitiveWords = []string{
	"微信号", "加我微信", "联系电话",
	"http://", "https://",
}

// ScanSensitive 扫描文本中的敏感词，返回所有命中位置。
// 返回 nil 表示无命中。
func ScanSensitive(text string) []*pb.SensitiveWordHit {
	if text == "" {
		return nil
	}
	var hits []*pb.SensitiveWordHit
	lower := strings.ToLower(text)
	for _, word := range sensitiveWords {
		w := strings.ToLower(word)
		start := 0
		for {
			idx := strings.Index(lower[start:], w)
			if idx < 0 {
				break
			}
			abs := start + idx
			hits = append(hits, &pb.SensitiveWordHit{
				Word:   word,
				Start:  int32(abs),
				End:    int32(abs + len(w)),
				Length: int32(len(w)),
			})
			start = abs + len(w)
		}
	}
	return hits
}