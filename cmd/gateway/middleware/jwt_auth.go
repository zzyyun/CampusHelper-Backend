package middleware

import (
	"errors"
	"strings"

	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/errcode"
	pkgjwt "go_projects/praProject1/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserID = "user_id"
	CtxRole   = "user_role"
)

// JWTAuth 校验 Bearer Token，把 user_id / role / school_id 写入 gin.Context。
//
// 错误响应统一为 {code, message, trace_id}：
//   - 缺失或格式错误 → 20001 missing token
//   - 过期           → 20002 token expired
//   - 其他无效       → 20003 invalid token
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			ErrorResponse(c, errcode.ErrMissingToken, "缺少 token")
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := pkgjwt.ParseAccessToken(tokenStr, config.Conf.Jwt.AuthKey)
		if err != nil {
			if errors.Is(err, pkgjwt.ErrAccessTokenExpired) {
				ErrorResponse(c, errcode.ErrTokenExpired, "token 已过期")
				return
			}
			ErrorResponse(c, errcode.ErrInvalidToken, "token 无效")
			return
		}

		c.Set(CtxUserID, claims.UserID)     // int64
		c.Set(CtxRole, claims.Role)         // int8
		c.Set(CtxSchoolID, claims.SchoolID) // int64：供 RequireSchoolBound 与下游 metadata 注入使用
		c.Next()
	}
}

// RequireRole 在已认证上下文中校验角色等级，不足则返回 20007 权限不足。
func RequireRole(minRole int8) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(CtxRole)
		r, _ := role.(int8)
		if r < minRole {
			ErrorResponse(c, errcode.ErrInsufficientPermission, "权限不足")
			return
		}
		c.Next()
	}
}