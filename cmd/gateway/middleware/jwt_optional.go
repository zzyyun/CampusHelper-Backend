package middleware

import (
	"strings"

	"go_projects/praProject1/config"
	pkgjwt "go_projects/praProject1/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// OptionalJWTAuth 可选 JWT 鉴权中间件（游客模式）。
//
// 与 JWTAuth 的区别：
//   - 有合法 Token  → 与 JWTAuth 行为一致，注入 user_id / role / school_id
//   - 缺失或无效 Token → 注入零值（user_id=0, school_id=0, role=0）并放行，不返回错误
//
// 适用场景：游客可浏览的读接口（帖子列表、任务列表、搜索等）。
func OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			// 无 Token，以游客身份放行
			setGuestContext(c)
			c.Next()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := pkgjwt.ParseAccessToken(tokenStr, config.Conf.Jwt.AuthKey)
		if err != nil {
			// Token 无效或过期，以游客身份放行（不报错）
			setGuestContext(c)
			c.Next()
			return
		}

		// Token 合法，注入完整身份
		c.Set(CtxUserID, claims.UserID)     // int64
		c.Set(CtxRole, claims.Role)         // int8
		c.Set(CtxSchoolID, claims.SchoolID) // int64
		c.Next()
	}
}

// setGuestContext 注入零值身份，确保后续 handler 可安全读取 context 键。
func setGuestContext(c *gin.Context) {
	c.Set(CtxUserID, int64(0))
	c.Set(CtxRole, int8(0))
	c.Set(CtxSchoolID, int64(0))
}
