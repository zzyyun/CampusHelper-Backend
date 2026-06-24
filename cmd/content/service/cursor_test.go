package service

import (
	"encoding/base64"
	"testing"
)

// ─── encodeCursor 测试 ────────────────────────────────────────────────────────

func TestEncodeCursor_Valid(t *testing.T) {
	encoded := encodeCursor(12345)
	if encoded == "" {
		t.Fatal("encodeCursor(12345) 不应返回空字符串")
	}

	// 解码验证
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if string(data) != `{"last_id":12345}` {
		t.Errorf("期望 {\"last_id\":12345}，实际 %s", string(data))
	}
}

func TestEncodeCursor_Zero(t *testing.T) {
	encoded := encodeCursor(0)
	if encoded != "" {
		t.Errorf("encodeCursor(0) 应返回空字符串，实际 %q", encoded)
	}
}

func TestEncodeCursor_Negative(t *testing.T) {
	encoded := encodeCursor(-1)
	if encoded != "" {
		t.Errorf("encodeCursor(-1) 应返回空字符串，实际 %q", encoded)
	}
}

// ─── parseCursor 测试 ────────────────────────────────────────────────────────

func TestParseCursor_Empty(t *testing.T) {
	if id := parseCursor(""); id != 0 {
		t.Errorf("空字符串应返回 0，实际 %d", id)
	}
}

func TestParseCursor_Base64Format(t *testing.T) {
	cursor := encodeCursor(99999)
	id := parseCursor(cursor)
	if id != 99999 {
		t.Errorf("Base64 游标解析后应为 99999，实际 %d", id)
	}
}

func TestParseCursor_LegacyDecimal(t *testing.T) {
	// 向后兼容：旧版纯数字格式
	id := parseCursor("777")
	if id != 777 {
		t.Errorf("旧版纯数字 777 应返回 777，实际 %d", id)
	}
}

func TestParseCursor_InvalidBase64(t *testing.T) {
	// 不是合法 Base64 也不是数字 → 返回 0
	id := parseCursor("!!!invalid!!!")
	if id != 0 {
		t.Errorf("非法游标应返回 0，实际 %d", id)
	}
}

func TestParseCursor_Base64ButInvalidJSON(t *testing.T) {
	// 合法 Base64 但 JSON 格式不对
	badJSON := base64.StdEncoding.EncodeToString([]byte(`not json`))
	id := parseCursor(badJSON)
	if id != 0 {
		t.Errorf("合法 Base64 但非法 JSON 应返回 0，实际 %d", id)
	}
}

func TestParseCursor_Base64WithZeroID(t *testing.T) {
	// JSON 格式正确但 last_id=0
	zeroID := base64.StdEncoding.EncodeToString([]byte(`{"last_id":0}`))
	id := parseCursor(zeroID)
	if id != 0 {
		t.Errorf("last_id=0 应返回 0，实际 %d", id)
	}
}

// ─── 往返测试 ────────────────────────────────────────────────────────────────

func TestCursor_RoundTrip(t *testing.T) {
	tests := []int64{1, 100, 999999999, 1 << 50}
	for _, want := range tests {
		encoded := encodeCursor(want)
		decoded := parseCursor(encoded)
		if decoded != want {
			t.Errorf("往返失败: encode(%d)=%q → parse=%d", want, encoded, decoded)
		}
	}
}
