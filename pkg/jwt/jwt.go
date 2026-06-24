package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v4"
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
//
// 参数约定：
//   - userID  / role：必填，调用方从 user model 读取
//   - schoolID：用户当前绑定的学校 ID，未绑定时传 0
//   - secret  / expireH：来自 config.Jwt 配置
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
//   - 过期：返回携带 jwtlib.ErrTokenExpired 的 error（用 errors.Is 判断）
//   - 签名错误 / Claims 类型不匹配 / 格式错误：返回 errors.New("invalid token")
//   - 旧版 token（不包含 school_id 字段）反序列化时 SchoolID 自动为 0，向后兼容
func ParseToken(tokenStr, secret string) (*UserClaims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}