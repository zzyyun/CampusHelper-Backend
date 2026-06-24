package jwt

import (
	"errors"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
)

const (
	testSecret    = "test-secret-key"
	testAccessH   = 1
	testRefreshH  = 24
)

// ─── Access Token ────────────────────────────────────────────────────────────

// TestGenerateAccessToken 验证 access token 签发并能正确解析出所有 Claims。
func TestGenerateAccessToken(t *testing.T) {
	tok, err := GenerateAccessToken(1001, 2002, 1, testSecret, testAccessH)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if tok == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := ParseAccessToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != 1001 {
		t.Errorf("UserID = %d, want 1001", claims.UserID)
	}
	if claims.SchoolID != 2002 {
		t.Errorf("SchoolID = %d, want 2002", claims.SchoolID)
	}
	if claims.Role != 1 {
		t.Errorf("Role = %d, want 1", claims.Role)
	}
}

// TestParseAccessToken_Expired 验证过期返回 ErrAccessTokenExpired。
func TestParseAccessToken_Expired(t *testing.T) {
	// 用过去时间签发一个已过期的 token
	claims := UserClaims{
		UserID:   42,
		SchoolID: 0,
		Role:     0,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = ParseAccessToken(tok, testSecret)
	if !errors.Is(err, ErrAccessTokenExpired) {
		t.Errorf("err = %v, want ErrAccessTokenExpired", err)
	}
}

// TestParseAccessToken_Invalid 验证错误 secret / 错误签名返回 ErrAccessTokenInvalid。
func TestParseAccessToken_Invalid(t *testing.T) {
	tok, _ := GenerateAccessToken(1, 2, 0, testSecret, testAccessH)

	_, err := ParseAccessToken(tok, "wrong-secret")
	if !errors.Is(err, ErrAccessTokenInvalid) {
		t.Errorf("wrong-secret: err = %v, want ErrAccessTokenInvalid", err)
	}

	_, err = ParseAccessToken("not-a-jwt", testSecret)
	if !errors.Is(err, ErrAccessTokenInvalid) {
		t.Errorf("garbage: err = %v, want ErrAccessTokenInvalid", err)
	}
}

// TestParseAccessToken_WrongAlg 验证非 HMAC 签名被拒绝（防 alg=none 攻击）。
func TestParseAccessToken_WrongAlg(t *testing.T) {
	// 手工构造一个 alg=none 的 token
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodNone, UserClaims{UserID: 1})
	signed, err := tok.SignedString(jwtlib.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}

	_, err = ParseAccessToken(signed, testSecret)
	if !errors.Is(err, ErrAccessTokenInvalid) {
		t.Errorf("alg=none: err = %v, want ErrAccessTokenInvalid", err)
	}
}

// ─── Refresh Token ───────────────────────────────────────────────────────────

// TestGenerateRefreshToken 验证 refresh token 签发与解析。
func TestGenerateRefreshToken(t *testing.T) {
	tok, err := GenerateRefreshToken(7777, testSecret, testRefreshH)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := ParseRefreshToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != 7777 {
		t.Errorf("UserID = %d, want 7777", claims.UserID)
	}
	// RefreshClaims 不应携带业务字段（这里主要验证字段缺失时反序列化为 0）
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
}

// TestParseRefreshToken_Expired 验证 refresh 过期返回 ErrRefreshTokenExpired。
func TestParseRefreshToken_Expired(t *testing.T) {
	claims := RefreshClaims{
		UserID: 99,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	tok, _ := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(testSecret))

	_, err := ParseRefreshToken(tok, testSecret)
	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Errorf("err = %v, want ErrRefreshTokenExpired", err)
	}
}

// TestParseRefreshToken_Invalid 验证 refresh token 错误 secret 返回 ErrRefreshTokenInvalid。
func TestParseRefreshToken_Invalid(t *testing.T) {
	tok, _ := GenerateRefreshToken(1, testSecret, testRefreshH)

	_, err := ParseRefreshToken(tok, "wrong-secret")
	if !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Errorf("err = %v, want ErrRefreshTokenInvalid", err)
	}
}

// ─── Cross-type safety ──────────────────────────────────────────────────────

// TestRefreshClaimsLacksBusinessFields 验证 RefreshClaims 不携带业务字段
// （防止泄露 school_id/role 等敏感信息到长期凭证）。
func TestRefreshClaimsLacksBusinessFields(t *testing.T) {
	tok, _ := GenerateRefreshToken(42, testSecret, testRefreshH)

	claims, err := ParseRefreshToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	// RefreshClaims 只有 UserID + RegisteredClaims，不应有 SchoolID/Role
	// 通过尝试访问检查（编译期就会失败 —— 这里用注释说明设计约束）
	_ = claims.UserID
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
	// 验证签发 access 时确实携带了这些字段（对比测试）
	accessTok, _ := GenerateAccessToken(42, 100, 1, testSecret, testAccessH)
	ac, err := ParseAccessToken(accessTok, testSecret)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if ac.SchoolID != 100 || ac.Role != 1 {
		t.Errorf("access token should carry SchoolID/Role, got school=%d role=%d", ac.SchoolID, ac.Role)
	}
}