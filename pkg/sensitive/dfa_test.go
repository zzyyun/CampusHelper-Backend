package sensitive

import (
	"testing"
)

// ─── Trie 构建 ──────────────────────────────────────────────────────────────

// TestDFAMatcher_Build 验证 Trie 树构建后 root 节点结构正确。
func TestDFAMatcher_Build(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"微信", "微信号", "广告"})

	if len(m.root.Children) == 0 {
		t.Fatal("root 节点 Children 不应为空")
	}
	// "微" 节点应存在
	weiNode, ok := m.root.Children['微']
	if !ok {
		t.Fatal("未找到 '微' 节点")
	}
	// "信" 是 "微" 的子节点
	xinNode, ok := weiNode.Children['信']
	if !ok {
		t.Fatal("未找到 '信' 节点")
	}
	if !xinNode.IsEnd || xinNode.Word != "微信" {
		t.Errorf("'微信' 节点 IsEnd=%v Word=%q", xinNode.IsEnd, xinNode.Word)
	}
	// "微信号" 是 "信" 的后继
	haoNode, ok := xinNode.Children['号']
	if !ok {
		t.Fatal("未找到 '号' 节点")
	}
	if !haoNode.IsEnd || haoNode.Word != "微信号" {
		t.Errorf("'微信号' 节点 IsEnd=%v Word=%q", haoNode.IsEnd, haoNode.Word)
	}
}

// TestDFAMatcher_Build_Empty 验证空词表不会导致 panic。
func TestDFAMatcher_Build_Empty(t *testing.T) {
	m := NewDFAMatcher()
	m.Build(nil)
	if len(m.root.Children) != 0 {
		t.Error("空词表构建后 Children 应为空")
	}
	m.Build([]string{})
	if len(m.root.Children) != 0 {
		t.Error("空切片构建后 Children 应为空")
	}
}

// TestDFAMatcher_Build_SkipEmpty 验证空白词被跳过。
func TestDFAMatcher_Build_SkipEmpty(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"", "  ", "test"})
	if len(m.root.Children) != 1 {
		t.Errorf("空白词应被跳过，Children 数量期望 1，实际 %d", len(m.root.Children))
	}
}

// ─── Match ──────────────────────────────────────────────────────────────────

// TestDFAMatcher_Match_Empty 验证空文本无命中。
func TestDFAMatcher_Match_Empty(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"敏感词"})

	if got := m.Match(""); got != nil {
		t.Fatalf("空文本应返回 nil，实际 %v", got)
	}
}

// TestDFAMatcher_Match_NoHit 验证无敏感词文本返回 nil。
func TestDFAMatcher_Match_NoHit(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"微信", "广告", "违法"})

	got := m.Match("今天天气真好，适合出去玩。")
	if got != nil {
		t.Fatalf("正常文本应返回 nil，实际 %v", got)
	}
}

// TestDFAMatcher_Match_SingleHit 验证单次命中并检查位置信息。
func TestDFAMatcher_Match_SingleHit(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"微信号"})

	// "请联系我微信号" → "请联系我" = 4 中文字符 = 12 字节
	hits := m.Match("请联系我微信号")
	if len(hits) != 1 {
		t.Fatalf("期望 1 个命中，实际 %d", len(hits))
	}
	if hits[0].Word != "微信号" {
		t.Errorf("Word = %q，期望 '微信号'", hits[0].Word)
	}
	if hits[0].Start != 12 {
		t.Errorf("Start = %d，期望 12（字节偏移）", hits[0].Start)
	}
	if hits[0].Length != 9 {
		t.Errorf("Length = %d，期望 9（字节长度）", hits[0].Length)
	}
}

// TestDFAMatcher_Match_Multiple 验证多模式同时命中。
func TestDFAMatcher_Match_Multiple(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"http://", "联系电话", "微信号"})

	// 文本同时含 3 个敏感词
	hits := m.Match("详情见 http://example.com 联系电话 13800001111")
	if len(hits) < 2 {
		t.Fatalf("期望至少 2 个命中，实际 %d", len(hits))
	}
	words := make(map[string]bool)
	for _, h := range hits {
		words[h.Word] = true
	}
	if !words["http://"] {
		t.Error("未命中 'http://'")
	}
	if !words["联系电话"] {
		t.Error("未命中 '联系电话'")
	}
}

// TestDFAMatcher_Match_Overlap 验证重叠模式匹配（"微信" 和 "微信号" 都应命中）。
func TestDFAMatcher_Match_Overlap(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"微信", "微信号"})

	hits := m.Match("请联系我的微信号")
	if len(hits) < 2 {
		t.Fatalf("重叠模式应命中 2 次，实际 %d", len(hits))
	}
	words := make(map[string]bool)
	for _, h := range hits {
		words[h.Word] = true
	}
	if !words["微信"] {
		t.Error("未命中 '微信'")
	}
	if !words["微信号"] {
		t.Error("未命中 '微信号'")
	}
}

// TestDFAMatcher_Match_English 验证英文敏感词匹配。
func TestDFAMatcher_Match_English(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"test", "spam"})

	hits := m.Match("this is a test message")
	if len(hits) != 1 {
		t.Fatalf("期望 1 个命中，实际 %d", len(hits))
	}
	if hits[0].Word != "test" {
		t.Errorf("Word = %q，期望 'test'", hits[0].Word)
	}
}

// ─── MatchCount ─────────────────────────────────────────────────────────────

// TestDFAMatcher_MatchCount 验证计数模式正确性。
func TestDFAMatcher_MatchCount(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"微信", "微信号", "广告"})

	text := "加我微信看广告，微信号在简介"
	// "加我微信看广告，微信号在简介"
	// 命中：微信(位置2)、广告(位置5)、微信(位置8, 微信号前缀)、微信号(位置8)
	if got := m.MatchCount(text); got != 4 {
		t.Errorf("MatchCount = %d，期望 4", got)
	}
	// 空文本
	if got := m.MatchCount(""); got != 0 {
		t.Errorf("空文本 MatchCount = %d，期望 0", got)
	}
}

// ─── 并发安全 ──────────────────────────────────────────────────────────────

// TestDFAMatcher_ConcurrentAccess 验证并发 Match 不会 panic。
func TestDFAMatcher_ConcurrentAccess(t *testing.T) {
	m := NewDFAMatcher()
	m.Build([]string{"微信", "广告", "违法", "敏感", "测试"})

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.Match("这是一段包含微信和广告的测试文本")
				m.MatchCount("违法敏感词测试")
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ─── 基准测试 ──────────────────────────────────────────────────────────────

// BenchmarkDFAMatcher_Match 基准：10000 字文本扫描。
func BenchmarkDFAMatcher_Match(b *testing.B) {
	m := NewDFAMatcher()
	// 构建 100 词词表
	words := make([]string, 100)
	for i := 0; i < 100; i++ {
		words[i] = "敏感词" + string(rune('A'+i%26))
	}
	m.Build(words)

	// 构造 10000 字中文文本
	text := make([]rune, 10000)
	for i := range text {
		text[i] = '测'
	}
	// 在中间位置插入一个敏感词
	copy(text[5000:], []rune("敏感词A"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(string(text))
	}
}