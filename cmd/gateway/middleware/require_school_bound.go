package middleware

import (
	"go_projects/praProject1/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// RequireSchoolBound 拒绝未绑定学校的用户访问受保护接口。
//
// 行为约定：
//   - 必须在 JWTAuth 之后执行（依赖 CtxSchoolID）
//   - 当 CtxSchoolID == 0 时返回 403 + {code: 20006, message: "campus not bound"}
//   - 已在下游 gRPC 双向保护：handler authCtx 注入 school-id metadata，未绑定调用会
//     在网关层就被拒绝
//
// 适用场景：
//   - 发帖、评论、点赞等"写"接口（POST/PUT/DELETE）
//   - 不适用于"读"接口（GET）：未绑定用户应能浏览
func RequireSchoolBound() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, exists := c.Get(CtxSchoolID)
		if !exists {
			// JWTAuth 未执行或执行失败，按 401 处理
			ErrorResponse(c, errcode.ErrMissingToken, "缺少用户身份")
			return
		}
		schoolID, ok := v.(int64)
		if !ok {
			ErrorResponse(c, errcode.ErrInvalidToken, "学校标识类型错误")
			return
		}
		if schoolID == 0 {
			ErrorResponse(c, errcode.ErrCampusNotBound, "未绑定学校，无法访问该接口")
			return
		}
		c.Next()
	}
}