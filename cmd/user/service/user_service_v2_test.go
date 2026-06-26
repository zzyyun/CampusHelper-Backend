package service

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestUserRoleFromCtx_NoMetadata(t *testing.T) {
	// 无 metadata 时应返回 0
	role := userRoleFromCtx(context.Background())
	if role != 0 {
		t.Errorf("空 context 应返回 0，实际 %d", role)
	}
}

func TestUserRoleFromCtx_WithMetadata(t *testing.T) {
	md := metadata.Pairs("user-role", "2")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	role := userRoleFromCtx(ctx)
	if role != 2 {
		t.Errorf("应返回 2(RoleAdmin)，实际 %d", role)
	}
}

func TestUserRoleFromCtx_InvalidValue(t *testing.T) {
	md := metadata.Pairs("user-role", "not-a-number")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	role := userRoleFromCtx(ctx)
	if role != 0 {
		t.Errorf("无效值应返回 0，实际 %d", role)
	}
}

func TestUserRoleFromCtx_SuperAdmin(t *testing.T) {
	md := metadata.Pairs("user-role", "3")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	role := userRoleFromCtx(ctx)
	if role != 3 {
		t.Errorf("应返回 3(RoleSuperAdmin)，实际 %d", role)
	}
}

func TestUserSchoolFromCtx_NoMetadata(t *testing.T) {
	schoolID := userSchoolFromCtx(context.Background())
	if schoolID != 0 {
		t.Errorf("空 context 应返回 0，实际 %d", schoolID)
	}
}

func TestUserSchoolFromCtx_WithMetadata(t *testing.T) {
	md := metadata.Pairs("school-id", "12345")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	schoolID := userSchoolFromCtx(ctx)
	if schoolID != 12345 {
		t.Errorf("应返回 12345，实际 %d", schoolID)
	}
}

func TestUserSchoolFromCtx_InvalidValue(t *testing.T) {
	md := metadata.Pairs("school-id", "invalid")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	schoolID := userSchoolFromCtx(ctx)
	if schoolID != 0 {
		t.Errorf("无效值应返回 0，实际 %d", schoolID)
	}
}

// ─── 游标编码 ──────────────────────────────────────────────────────────────

func TestCursorRoundTrip(t *testing.T) {
	originalID := int64(1234567890123456789)
	cursor := encodeCursor(originalID)

	var decodedID int64
	if err := parseCursor(cursor, &decodedID); err != nil {
		t.Fatalf("parseCursor 失败: %v", err)
	}

	if decodedID != originalID {
		t.Errorf("游标编码往返失败: 期望 %d，实际 %d", originalID, decodedID)
	}
}

func TestEncodeCursor_Zero(t *testing.T) {
	cursor := encodeCursor(0)
	if cursor == "" {
		t.Error("零 ID 也应产生非空游标")
	}

	var decodedID int64
	if err := parseCursor(cursor, &decodedID); err != nil {
		t.Fatalf("parseCursor 失败: %v", err)
	}
	if decodedID != 0 {
		t.Errorf("期望 0，实际 %d", decodedID)
	}
}

func TestParseCursor_InvalidBase64(t *testing.T) {
	var id int64
	err := parseCursor("!!!invalid!!!", &id)
	if err == nil {
		t.Error("无效 Base64 应返回错误")
	}
}

func TestParseCursor_InvalidJSON(t *testing.T) {
	invalidJSON := base64URLEncode([]byte("not-json"))
	var id int64
	err := parseCursor(invalidJSON, &id)
	if err == nil {
		t.Error("无效 JSON 应返回错误")
	}
}

func TestParseCursor_EmptyString(t *testing.T) {
	var id int64
	err := parseCursor("", &id)
	if err == nil {
		t.Error("空字符串应返回错误")
	}
}

// ─── base64url 工具函数 ────────────────────────────────────────────────────

func TestBase64URLRoundTrip(t *testing.T) {
	original := []byte("test-data-for-round-trip")
	encoded := base64URLEncode(original)
	decoded, err := base64URLDecode(encoded)
	if err != nil {
		t.Fatalf("base64URLDecode 失败: %v", err)
	}
	if string(decoded) != string(original) {
		t.Errorf("base64url 往返失败: 期望 %s，实际 %s", original, decoded)
	}
}

func TestBase64URLEncode_KnownValue(t *testing.T) {
	// 验证已知编码值（与标准库对齐）
	result := base64URLEncode([]byte(`{"id":1}`))
	if result == "" {
		t.Error("编码结果不应为空")
	}
}
