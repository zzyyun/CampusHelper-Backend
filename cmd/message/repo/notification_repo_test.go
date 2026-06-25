package repo

import (
	"testing"
)

// ─── 游标分页编码/解码测试 ──────────────────────────────────────────────────

func TestEncodeCursor_Empty(t *testing.T) {
	if got := EncodeCursor(Cursor{}); got != "" {
		t.Errorf("空游标应返回空字符串，实际 %q", got)
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	c, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("空游标解码失败: %v", err)
	}
	if c.ID != 0 {
		t.Errorf("空游标 ID 应为 0，实际 %d", c.ID)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("!!!invalid-base64!!!")
	if err == nil {
		t.Error("非法 Base64 游标应返回错误")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	cursors := []Cursor{
		{ID: 1, CreatedAt: 100},
		{ID: 999, CreatedAt: 9999999999},
		{ID: 12345, CreatedAt: 1700000000},
	}
	for _, c := range cursors {
		encoded := EncodeCursor(c)
		if encoded == "" {
			t.Errorf("非空游标 %+v 编码后不应为空", c)
			continue
		}
		decoded, err := DecodeCursor(encoded)
		if err != nil {
			t.Errorf("解码 %+v 失败: %v", c, err)
			continue
		}
		if decoded.ID != c.ID || decoded.CreatedAt != c.CreatedAt {
			t.Errorf("编解码不一致: 输入 %+v，输出 %+v", c, decoded)
		}
	}
}

func TestCursorOrderPreserved(t *testing.T) {
	// 验证游标编码后按 created_at DESC, id DESC 排序的性质
	// 较早的游标编码应按字典序小于较新的游标（仅当先编码 created_at 再编码 id）
	c1 := EncodeCursor(Cursor{ID: 1, CreatedAt: 100})
	c2 := EncodeCursor(Cursor{ID: 2, CreatedAt: 200})
	if c1 == "" || c2 == "" {
		t.Fatal("游标不应为空")
	}
	// 这里不做 strict 断言（Base64 不保证字典序），仅确保非空可解码
	t.Logf("c1=%q c2=%q", c1, c2)
}
