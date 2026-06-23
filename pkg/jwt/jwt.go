package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
)

// Sentinel 错误，调用方可通过 errors.Is 区分"过期"与"其他无效"。
var (
	// ErrTokenExpired token 已过期。
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenInvalid token 签名无效、格式错误或 Claims 类型不匹配。
	ErrTokenInvalid = errors.New("token invalid")
)

// UserClaims JWT 自定义 Claims，当前承载用户标识与角色。
//
// 注意：SchoolID 字段由 Issue #18 引入，本文件不修改结构，避免超出 Issue #17 范围。
type UserClaims struct {
	UserID int64 `json:"user_id"`
	Role   int8  `json:"role"`
	jwtlib.RegisteredClaims
}

// GenerateToken 签发 JWT。expireH 为过期小时数。
func GenerateToken(userID int64, role int8, secret string, expireH int) (string, error) {
	claims := UserClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Duration(expireH) * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	return jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken 解析并校验 JWT。
//
// 错误返回约定：
//   - 过期：返回 ErrTokenExpired（可被 errors.Is 识别）
//   - 签名错误、Claims 类型不匹配、格式错误等：返回 ErrTokenInvalid
//   - 业务侧只需 errors.Is(err, ErrTokenExpired) 即可区分两种情况
func ParseToken(tokenStr, secret string) (*UserClaims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		// jwtlib 过期错误优先识别；其他错误一律归为"无效"
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}