package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
)

// Sentinel 错误，调用方可通过 errors.Is 区分"过期"与"其他无效"。
var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// UserClaims JWT 自定义 Claims，承载用户标识、角色、学校三要素。
//
// 设计说明：
//   - UserID / Role / SchoolID 三个字段覆盖 Gateway 鉴权 + 多租户隔离的最小集合
//   - SchoolID = 0 表示用户尚未绑定学校，网关侧将拒绝其访问受保护写接口
//   - 字段保持扁平 JSON 序列化，兼容旧版 token（缺 school_id 时反序列化为 0）
type UserClaims struct {
	UserID   int64 `json:"user_id"`
	SchoolID int64 `json:"school_id"`
	Role     int8  `json:"role"`
	jwtlib.RegisteredClaims
}

// GenerateToken 签发 JWT。
func GenerateToken(userID, schoolID int64, role int8, secret string, expireH int) (string, error) {
	claims := UserClaims{
		UserID:   userID,
		SchoolID: schoolID,
		Role:     role,
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
//   - 其他无效：返回 ErrTokenInvalid
func ParseToken(tokenStr, secret string) (*UserClaims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
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