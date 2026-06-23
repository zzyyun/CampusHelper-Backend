package service

import (
	"reflect"
	"testing"

	pb "go_projects/praProject1/PB/pb/content_pb"
)

// 确保 import 用上
var _ = pb.SensitiveWordHit{}

func TestScanSensitive_Empty(t *testing.T) {
	if got := ScanSensitive(""); got != nil {
		t.Fatalf("空文本应返回 nil，实际为 %v", got)
	}
}

func TestScanSensitive_NoHit(t *testing.T) {
	text := "我今天在图书馆找到一本好书，分享给大家。"
	if got := ScanSensitive(text); got != nil {
		t.Fatalf("正常文本应无命中，实际为 %v", got)
	}
}

func TestScanSensitive_OneHit(t *testing.T) {
	text := "请联系我微信号 abc123"
	hits := ScanSensitive(text)
	if len(hits) != 1 {
		t.Fatalf("期望 1 个命中，实际为 %d", len(hits))
	}
	if hits[0].Word != "微信号" {
		t.Fatalf("期望命中词 '微信号'，实际为 %q", hits[0].Word)
	}
	// "请联系我" = 4 个中文 = 12 字节，"微信号" 从第 12 字节开始
	if hits[0].Start != 12 {
		t.Fatalf("期望 start=12 (字节偏移)，实际为 %d", hits[0].Start)
	}
	if hits[0].End != 12+int32(len("微信号")) {
		t.Fatalf("期望 end=%d，实际为 %d", 12+len("微信号"), hits[0].End)
	}
	if hits[0].Length != int32(len("微信号")) {
		t.Fatalf("期望 length=%d，实际为 %d", len("微信号"), hits[0].Length)
	}
}

func TestScanSensitive_MultipleHits(t *testing.T) {
	// 同时含 "http://" 和 "联系电话" 两个敏感词
	text := "详情请见 http://example.com 联系电话 13800001111"
	hits := ScanSensitive(text)
	if len(hits) < 2 {
		t.Fatalf("期望至少 2 个命中，实际为 %d", len(hits))
	}
	words := make(map[string]bool)
	for _, h := range hits {
		words[h.Word] = true
	}
	if !words["http://"] {
		t.Fatalf("未命中 'http://'")
	}
	if !words["联系电话"] {
		t.Fatalf("未命中 '联系电话'")
	}
}

func TestScanSensitive_HitType(t *testing.T) {
	// 验证返回类型是 *pb.SensitiveWordHit
	hits := ScanSensitive("微信号")
	if len(hits) == 0 {
		t.Fatal("期望至少 1 个命中")
	}
	if reflect.TypeOf(hits[0]).String() != "*content_pb.SensitiveWordHit" {
		t.Fatalf("类型错误: %v", reflect.TypeOf(hits[0]))
	}
}

func TestAsSensitiveWordError(t *testing.T) {
	hits := []*pb.SensitiveWordHit{{Word: "test", Start: 0, End: 4, Length: 4}}
	err := &SensitiveWordErrorType{Hits: hits}

	got, ok := AsSensitiveWordError(err)
	if !ok {
		t.Fatal("AsSensitiveWordError 应返回 ok=true")
	}
	if !reflect.DeepEqual(got.Hits, hits) {
		t.Fatalf("hits 不一致: %v vs %v", got.Hits, hits)
	}

	// 普通 error 应返回 false
	plainErr := error(&SensitiveWordErrorTypeShim{})
	if _, ok := AsSensitiveWordError(plainErr); ok {
		t.Fatal("普通 error 应返回 ok=false")
	}
}

// SensitiveWordErrorTypeShim 故意让 errors.As 失败的占位 error 类型
// （它实现了 Error() 但不是 *SensitiveWordErrorType）
type SensitiveWordErrorTypeShim struct{}

func (SensitiveWordErrorTypeShim) Error() string { return "shim" }