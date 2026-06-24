package jwt

import (
	"errors"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
)

const testSecret = "test-secret-key-for-jwt"

// TestGenerateAndParse_RoundTrip 验证新签发的 token 解析后所有字段（user_id / school_id / role）正确。
func TestGenerateAndParse_RoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		userID   int64
		schoolID int64
		role     int8
	}{
		{"normal user with school", 100, 12345, 1},
		{"user without school", 200, 0, 1},
		{"admin user", 300, 99999, 9},
		{"large school id", 400, 1<<62, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok, err := GenerateToken(tc.userID, tc.schoolID, tc.role, testSecret, 24)
			if err != nil {
				t.Fatalf("GenerateToken: %v", err)
			}
			claims, err := ParseToken(tok, testSecret)
			if err != nil {
				t.Fatalf("ParseToken: %v", err)
			}
			if claims.UserID != tc.userID {
				t.Errorf("UserID = %d, want %d", claims.UserID, tc.userID)
			}
			if claims.SchoolID != tc.schoolID {
				t.Errorf("SchoolID = %d, want %d", claims.SchoolID, tc.schoolID)
			}
			if claims.Role != tc.role {
				t.Errorf("Role = %d, want %d", claims.Role, tc.role)
			}
		})
	}
}

// TestParseToken_BackwardCompat 验证不带 school_id 字段的旧 token 仍可解析，SchoolID 自动为 0。
func TestParseToken_BackwardCompat(t *testing.T) {
	// 模拟旧版 token：手工构造不含 school_id 的 Claims
	type oldClaims struct {
		UserID int64 `json:"user_id"`
		Role   int8  `json:"role"`
		jwtlib.RegisteredClaims
	}
	old := oldClaims{
		UserID: 42,
		Role:   1,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, old).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign old token: %v", err)
	}

	claims, err := ParseToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", claims.UserID)
	}
	if claims.SchoolID != 0 {
		t.Errorf("SchoolID = %d, want 0 (old token has no school_id)", claims.SchoolID)
	}
	if claims.Role != 1 {
		t.Errorf("Role = %d, want 1", claims.Role)
	}
}

// TestParseToken_Expired 验证过期 token 返回 jwtlib.ErrTokenExpired 包装的错误。
func TestParseToken_Expired(t *testing.T) {
	// 直接构造已过期的 token（用 0 小时过期，即立即过期）
	tok, err := GenerateToken(1, 2, 1, testSecret, 0)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	// 等 1 秒确保 jwtlib 解析时已判定过期
	time.Sleep(1 * time.Second)

	_, err = ParseToken(tok, testSecret)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !errors.Is(err, jwtlib.ErrTokenExpired) {
		t.Errorf("err = %v, want jwtlib.ErrTokenExpired", err)
	}
}

// TestParseToken_InvalidSecret 验证用错误密钥签名的 token 解析失败。
func TestParseToken_InvalidSecret(t *testing.T) {
	tok, _ := GenerateToken(1, 2, 1, "secret-A", 24)
	_, err := ParseToken(tok, "secret-B")
	if err == nil {
		t.Fatal("expected error for token signed with different secret, got nil")
	}
}

// TestParseToken_Malformed 验证完全无效的字符串返回错误。
func TestParseToken_Malformed(t *testing.T) {
	_, err := ParseToken("not-a-jwt", testSecret)
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

// TestParseToken_WrongAlg 验证使用非 HMAC 算法签名的 token 被拒绝（防止 alg=none 攻击）。
func TestParseToken_WrongAlg(t *testing.T) {
	// 用 HS256 签名但 Claims 类型不带 HMAC method 触发 unexpected signing method 分支
	// 直接构造一个带 unsigned 算法的 token
	claims := UserClaims{UserID: 1, Role: 1}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodNone, claims)
	str, err := tok.SignedString(jwtlib.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}
	_, err = ParseToken(str, testSecret)
	if err == nil {
		t.Fatal("expected error for none-signed token, got nil")
	}
}

// TestParseToken_Empty 验证空字符串返回错误。
func TestParseToken_Empty(t *testing.T) {
	_, err := ParseToken("", testSecret)
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

// TestGenerateToken_ExpiresAt 验证 token 的 ExpiresAt 与配置一致。
func TestGenerateToken_ExpiresAt(t *testing.T) {
	tok, err := GenerateToken(1, 2, 1, testSecret, 24)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	claims, err := ParseToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	// 验证 ExpiresAt 在未来 23~25 小时之间
	diff := time.Until(claims.ExpiresAt.Time)
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("ExpiresAt diff = %v, want ~24h", diff)
	}
}