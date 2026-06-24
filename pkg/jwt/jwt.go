// Package jwt 提供 Access Token 与 Refresh Token 的签发与解析。
//
// 设计目标
//   - 拆分 Access / Refresh 两套 Claims，避免 Refresh Token 携带多余字段
//   - 统一 Sentinel 错误，调用方通过 errors.Is 区分过期与非法
//   - HS256 对称签名；强制校验签名方法防止 alg=none 攻击
//
// 与项目其它部分的契约
//   - Access Token Claims：UserID / SchoolID / Role / 过期时间
//   - Refresh Token Claims：仅 UserID / 过期时间（不携带业务字段）
//   - User Service 在 WxLogin 时同步颁发两个 Token；用户在 Access 过期后用
//     Refresh 调 /user/refresh 换取新 Access
package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
)

// Sentinel 错误，调用方可通过 errors.Is 区分"过期"与"其他无效"。
//
// 区分两个 token 类型各自的过期/非法错误码，便于 Gateway 在 /user/refresh
// 路径返回 20004 / 20005，在受保护接口返回 20002 / 20003。
var (
	ErrAccessTokenExpired  = errors.New("access token expired")
	ErrAccessTokenInvalid  = errors.New("access token invalid")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
	ErrRefreshTokenInvalid = errors.New("refresh token invalid")
)

// UserClaims 是 Access Token 的自定义 Claims，承载用户三要素。
//
// 字段说明：
//   - UserID / Role / SchoolID 覆盖 Gateway 鉴权 + 多租户隔离的最小集合
//   - SchoolID = 0 表示用户尚未绑定学校，网关侧将拒绝其访问受保护写接口
//   - JSON 字段名保持与旧版兼容（缺 school_id 时反序列化为 0）
type UserClaims struct {
	UserID   int64 `json:"user_id"`
	SchoolID int64 `json:"school_id"`
	Role     int8  `json:"role"`
	jwtlib.RegisteredClaims
}

// RefreshClaims 是 Refresh Token 的最小 Claims，仅含 UserID。
//
// 设计原因：
//   - Refresh Token 不应携带业务字段（school_id/role），避免续签时使用过期状态
//   - UserID 是唯一标识；续签时 Gateway 透传到 User Service，User Service
//     重新查库获取最新学校/角色后再签发新 Access Token
type RefreshClaims struct {
	UserID int64 `json:"user_id"`
	jwtlib.RegisteredClaims
}

// GenerateAccessToken 签发 Access Token。
func GenerateAccessToken(userID, schoolID int64, role int8, secret string, expireH int) (string, error) {
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

// GenerateRefreshToken 签发 Refresh Token。
func GenerateRefreshToken(userID int64, secret string, expireH int) (string, error) {
	claims := RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Duration(expireH) * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	return jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseAccessToken 解析 Access Token。
//
// 错误返回约定：
//   - 过期：ErrAccessTokenExpired（可被 errors.Is 识别）
//   - 其他无效：ErrAccessTokenInvalid
func ParseAccessToken(tokenStr, secret string) (*UserClaims, error) {
	claims := &UserClaims{}
	if err := parseSignedToken(tokenStr, secret, claims); err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrAccessTokenExpired
		}
		return nil, ErrAccessTokenInvalid
	}
	return claims, nil
}

// ParseRefreshToken 解析 Refresh Token。
//
// 错误返回约定：
//   - 过期：ErrRefreshTokenExpired
//   - 其他无效：ErrRefreshTokenInvalid
func ParseRefreshToken(tokenStr, secret string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}
	if err := parseSignedToken(tokenStr, secret, claims); err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrRefreshTokenExpired
		}
		return nil, ErrRefreshTokenInvalid
	}
	return claims, nil
}

// parseSignedToken 通用解析：仅接受 HMAC 签名方法（防 alg=none 攻击）。
func parseSignedToken(tokenStr, secret string, claims jwtlib.Claims) error {
	_, err := jwtlib.ParseWithClaims(tokenStr, claims, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	return err
}